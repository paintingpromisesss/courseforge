package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type commitItem struct {
	Hash    string
	Subject string
	Body    string
	Type    string
	Scope   string
	Message string
}

func main() {
	tag := ""
	if len(os.Args) > 1 {
		tag = os.Args[1]
	}
	if tag == "" {
		tag = getEnv("GITHUB_REF_NAME", "")
	}
	if tag == "" {
		tag = currentTag()
	}

	prevTag := findPreviousTag(tag)

	commits, err := getCommitsBetween(prevTag, tag)
	if err != nil || len(commits) == 0 {
		fmt.Printf("## What's Changed\n\n- Release %s\n\n---\n\n## Что нового\n\n- Релиз %s\n", tag, tag)
		return
	}

	diffStat := getDiffStat(prevTag, tag)
	codeDiff := getCodeDiff(prevTag, tag)

	apiKey := getEnv("AI_API_KEY", getEnv("OPENAI_API_KEY", getEnv("GROQ_API_KEY", "")))
	baseURL := getEnv("AI_BASE_URL", "https://api.openai.com/v1")
	model := getEnv("AI_MODEL", "gpt-4o-mini")

	// Try generating with AI if an API key is configured
	if apiKey != "" {
		endpoint := baseURL
		if !strings.HasSuffix(endpoint, "/chat/completions") {
			endpoint = strings.TrimRight(endpoint, "/") + "/chat/completions"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		notes, err := generateWithAI(ctx, endpoint, apiKey, model, tag, prevTag, commits, diffStat, codeDiff)
		if err == nil && strings.TrimSpace(notes) != "" {
			fmt.Println(strings.TrimSpace(notes))
			return
		}
		fmt.Fprintf(os.Stderr, "ai changelog notice: %v (falling back to conventional commits)\n", err)
	}

	// Fallback: structured changelog from Conventional Commits
	fmt.Println(generateFallbackNotes(commits))
}

func currentTag() string {
	out, err := exec.Command("git", "describe", "--tags", "--exact-match").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	out, err = exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "HEAD"
}

func findPreviousTag(current string) string {
	out, err := exec.Command("git", "tag", "--sort=-creatordate").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	foundCurrent := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if current != "" && (t == current || strings.TrimPrefix(t, "refs/tags/") == current) {
			foundCurrent = true
			continue
		}
		if foundCurrent || current == "" {
			return t
		}
	}
	return ""
}

func getCommitsBetween(prev, current string) ([]commitItem, error) {
	rangeArg := current
	if prev != "" {
		rangeArg = fmt.Sprintf("%s..%s", prev, current)
	}

	cmd := exec.Command("git", "log", rangeArg, "--no-merges", "--pretty=format:%h%x1f%s%x1f%b%x1e")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var items []commitItem
	rawCommits := strings.Split(string(out), "\x1e")
	for _, raw := range rawCommits {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, "\x1f")
		if len(parts) < 2 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		subj := strings.TrimSpace(parts[1])
		body := ""
		if len(parts) > 2 {
			body = strings.TrimSpace(parts[2])
		}

		cType, scope, msg := parseConventional(subj)
		items = append(items, commitItem{
			Hash:    hash,
			Subject: subj,
			Body:    body,
			Type:    cType,
			Scope:   scope,
			Message: msg,
		})
	}
	return items, nil
}

func getDiffStat(prev, current string) string {
	rangeArg := current
	if prev != "" {
		rangeArg = fmt.Sprintf("%s..%s", prev, current)
	}
	out, err := exec.Command("git", "diff", "--stat", rangeArg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getCodeDiff(prev, current string) string {
	rangeArg := current
	if prev != "" {
		rangeArg = fmt.Sprintf("%s..%s", prev, current)
	}
	args := []string{
		"diff",
		rangeArg,
		"--",
		":(exclude)package-lock.json",
		":(exclude)go.sum",
		":(exclude)*.png",
		":(exclude)*.jpg",
		":(exclude)*.jpeg",
		":(exclude)*.svg",
		":(exclude)*.ico",
		":(exclude)*.woff",
		":(exclude)*.woff2",
		":(exclude)dist/*",
		":(exclude)bin/*",
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	s := string(out)
	// Generous 120 KB ceiling (~30k tokens) so massive files don't blow up context
	const maxChars = 120_000
	if len(s) > maxChars {
		s = s[:maxChars] + "\n\n... [diff truncated for length] ...\n"
	}
	return strings.TrimSpace(s)
}

func parseConventional(s string) (cType, scope, msg string) {
	prefix, after, found := strings.Cut(s, ":")
	if !found {
		return "other", "", s
	}
	prefix = strings.TrimSpace(prefix)
	msg = strings.TrimSpace(after)

	if open := strings.Index(prefix, "("); open != -1 && strings.HasSuffix(prefix, ")") {
		cType = strings.ToLower(prefix[:open])
		scope = prefix[open+1 : len(prefix)-1]
	} else {
		cType = strings.ToLower(prefix)
	}
	return cType, scope, msg
}

func generateWithAI(ctx context.Context, endpoint, apiKey, model, tag, prevTag string, commits []commitItem, diffStat, codeDiff string) (string, error) {
	var commitList strings.Builder
	for _, c := range commits {
		fmt.Fprintf(&commitList, "- %s: %s\n", c.Hash, c.Subject)
		if c.Body != "" {
			for _, line := range strings.Split(c.Body, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(&commitList, "    %s\n", line)
				}
			}
		}
	}

	systemPrompt := `You are the release manager for CourseForge, a self-hosted developer learning platform.
Generate clean, concise, user-friendly release notes from the provided git commits, changed files, and code diff.
Analyze the actual code changes and commit descriptions to understand what features, bugfixes, UI improvements, or architecture changes were introduced.
Focus on user impact and key technical components (e.g. self-updater, MCP server, runners, settings, UI/UX, database).
Group changes under standard section headings. Only include a section if there are relevant items.
Ignore trivial internal changes, documentation typos, or build chores unless they affect developers or users.
Do NOT use emojis anywhere in the output.

Output EXACTLY in this format with two sections separated by "---":

## What's Changed

### Features
- Item description

### Fixes
- Item description

### Improvements
- Item description

---

## Что нового

### Новые возможности
- Описание пункта

### Исправления
- Описание пункта

### Улучшения
- Описание пункта

Do not output code blocks or markdown fences around the response. Output plain markdown directly.`

	var promptBuilder strings.Builder
	fmt.Fprintf(&promptBuilder, "Release: %s (previous: %s)\n\n", tag, prevTag)
	fmt.Fprintf(&promptBuilder, "### Commits and descriptions:\n%s\n", commitList.String())
	if diffStat != "" {
		fmt.Fprintf(&promptBuilder, "### Changed Files (git diff --stat):\n%s\n\n", diffStat)
	}
	if codeDiff != "" {
		fmt.Fprintf(&promptBuilder, "### Code Diff:\n```diff\n%s\n```\n", codeDiff)
	}

	userPrompt := promptBuilder.String()

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
		"max_tokens":  2000,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ai api status %s: %s", resp.Status, string(respBody))
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("empty choices from ai response")
	}

	return res.Choices[0].Message.Content, nil
}

func generateFallbackNotes(commits []commitItem) string {
	featsEN, fixesEN, othersEN := []string{}, []string{}, []string{}
	featsRU, fixesRU, othersRU := []string{}, []string{}, []string{}

	for _, c := range commits {
		scope := ""
		if c.Scope != "" {
			scope = fmt.Sprintf("**%s**: ", c.Scope)
		}
		itemEN := fmt.Sprintf("- %s%s (%s)", scope, c.Message, c.Hash)
		itemRU := fmt.Sprintf("- %s%s (%s)", scope, c.Message, c.Hash)

		switch c.Type {
		case "feat":
			featsEN = append(featsEN, itemEN)
			featsRU = append(featsRU, itemRU)
		case "fix":
			fixesEN = append(fixesEN, itemEN)
			fixesRU = append(fixesRU, itemRU)
		default:
			othersEN = append(othersEN, itemEN)
			othersRU = append(othersRU, itemRU)
		}
	}

	var b strings.Builder
	b.WriteString("## What's Changed\n\n")
	if len(featsEN) > 0 {
		b.WriteString("### Features\n")
		for _, it := range featsEN {
			b.WriteString(it)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(fixesEN) > 0 {
		b.WriteString("### Fixes\n")
		for _, it := range fixesEN {
			b.WriteString(it)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(othersEN) > 0 {
		b.WriteString("### Other Changes\n")
		for _, it := range othersEN {
			b.WriteString(it)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	b.WriteString("---\n\n## Что нового\n\n")
	if len(featsRU) > 0 {
		b.WriteString("### Новые возможности\n")
		for _, it := range featsRU {
			b.WriteString(it)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(fixesRU) > 0 {
		b.WriteString("### Исправления\n")
		for _, it := range fixesRU {
			b.WriteString(it)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(othersEN) > 0 {
		b.WriteString("### Прочие изменения\n")
		for _, it := range othersRU {
			b.WriteString(it)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	return strings.TrimSpace(b.String())
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
