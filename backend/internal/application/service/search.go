package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

// SearchRank classifies how strongly a task matched the query; lower is better.
type SearchRank int

const (
	RankTitleWord SearchRank = iota // whole-word (word-boundary) match in task title
	RankTag                         // match in task tags
	RankTitle                       // title contains the word as a substring
	RankContent                     // only the statement content matched
)

// SearchTask is the flattened, searchable view of one task in the library.
type SearchTask struct {
	Course    string
	CourseDir string // relative course dir (needed to locate statement.md on disk)
	Track     string
	Topic     string
	Unit      string
	Task      string
	Statement string // statement file name, relative to the task folder
	Title     string
	Tags      []string
	Difficulty int
}

// SearchResult is one row of the search output.
type SearchResult struct {
	Course     string   `json:"course"`
	Track      string   `json:"track"`
	Topic      string   `json:"topic"`
	Unit       string   `json:"unit"`
	Task       string   `json:"task"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags"`
	Difficulty int      `json:"difficulty,omitempty"`
	Rank       string   `json:"rank"` // title-word | tag | title | content
	Snippet    string   `json:"snippet,omitempty"`
}

// CatalogEntry is a lightweight task entry used to build the AI search prompt
// and hot-tag suggestions; it also carries enough path info for navigation.
type CatalogEntry struct {
	Course     string   `json:"course"`
	Track      string   `json:"track"`
	Topic      string   `json:"topic"`
	Unit       string   `json:"unit"`
	Task       string   `json:"task"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags,omitempty"`
	Difficulty int      `json:"difficulty,omitempty"`
}

// FlattenSearchTasks walks all loaded courses and builds the flat search index.
// It reads only in-memory structures — no manifest IO.
func FlattenSearchTasks(courses map[string]*domain.Course) []SearchTask {
	tasks := make([]SearchTask, 0, 256)
	for _, c := range courses {
		for _, tr := range c.Tracks {
			for _, tp := range tr.Topics {
				for _, u := range tp.Units {
					for _, t := range u.Tasks {
						tasks = append(tasks, SearchTask{
							Course:     c.Slug,
							CourseDir:  c.Dir,
							Track:      tr.Slug,
							Topic:      tp.Slug,
							Unit:       u.Slug,
							Task:       t.Slug,
							Statement:  t.Statement,
							Title:      t.Title,
							Tags:       t.Tags,
							Difficulty: t.Difficulty,
						})
					}
				}
			}
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Course < tasks[j].Course })
	return tasks
}

// BuildCatalog produces the compressed task catalog for the AI search prompt.
func BuildCatalog(tasks []SearchTask) []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(tasks))
	for _, t := range tasks {
		entries = append(entries, CatalogEntry{
			Course:     t.Course,
			Track:      t.Track,
			Topic:      t.Topic,
			Unit:       t.Unit,
			Task:       t.Task,
			Title:      t.Title,
			Tags:       t.Tags,
			Difficulty: t.Difficulty,
		})
	}
	return entries
}

// rankNames maps rank constants to their JSON representation.
func (r SearchRank) String() string {
	switch r {
	case RankTitleWord:
		return "title-word"
	case RankTag:
		return "tag"
	case RankTitle:
		return "title"
	default:
		return "content"
	}
}

// tokenize splits the query into lowercase words; empty tokens are dropped.
func tokenize(q string) []string {
	words := strings.Fields(q)
	out := make([]string, 0, len(words))
	for _, w := range words {
		if w = strings.ToLower(w); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// hasWordBoundary reports whether term occurs in s as a whole word
// (delimited by non-alphanumeric or string bounds), case-insensitive.
func hasWordBoundary(s, term string) bool {
	ls := strings.ToLower(s)
	for i := 0; ; {
		idx := strings.Index(ls[i:], term)
		if idx < 0 {
			return false
		}
		start := i + idx
		end := start + len(term)
		if !wordCharAt(ls, start-1) && !wordCharAt(ls, end) {
			return true
		}
		i = start + 1
	}
}

// wordCharAt reports whether the byte at position i is part of a word
// (ASCII letter/digit, or a UTF-8 continuation/lead byte of a non-ASCII
// runeful letter — an approximation sufficient for word boundaries).
func wordCharAt(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	if c >= '0' && c <= '9' {
		return true
	}
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	return c >= 0x80
}

// snippet extracts up to ~120 characters of content around the first
// case-insensitive occurrence of any term.
func snippet(content string, terms []string) string {
	lc := strings.ToLower(content)
	pos := -1
	for _, t := range terms {
		if i := strings.Index(lc, t); i >= 0 && (pos < 0 || i < pos) {
			pos = i
		}
	}
	if pos < 0 {
		return ""
	}
	const window = 120
	start := pos - 40
	if start < 0 {
		start = 0
	}
	end := pos + window - 40
	if end > len(content) {
		end = len(content)
	}
	// align to rune boundaries
	for start > 0 && !isRuneStart(content[start]) {
		start--
	}
	for end < len(content) && !isRuneStart(content[end]) {
		end++
	}
	s := strings.TrimSpace(strings.Join(strings.Fields(content[start:end]), " "))
	if start > 0 {
		s = "…" + s
	}
	if end < len(content) {
		s += "…"
	}
	return s
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// SearchTasks performs substring search over index fields with AND semantics
// between words. contentOf, when non-nil, is consulted for tasks whose
// metadata did not match — it lazily loads statement text (may be heavy).
// Every word must match somewhere: title, tags, content (statement title
// fields carry the path context via SearchTask fields).
func SearchTasks(
	tasks []SearchTask,
	query string,
	limit int,
	contentOf func(SearchTask) (string, bool),
) []SearchResult {
	terms := tokenize(query)
	if len(terms) == 0 || len([]rune(strings.TrimSpace(query))) < 2 {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}

	type scored struct {
		task    SearchTask
		rank    SearchRank
		snippet string
	}

	matches := make([]scored, 0, 8)
	for _, t := range tasks {
		rank, snip, ok := matchTask(t, terms, contentOf)
		if ok {
			matches = append(matches, scored{task: t, rank: rank, snippet: snip})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].rank != matches[j].rank {
			return matches[i].rank < matches[j].rank
		}
		if matches[i].task.Course != matches[j].task.Course {
			return matches[i].task.Course < matches[j].task.Course
		}
		return matches[i].task.Task < matches[j].task.Task
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}

	results := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		results = append(results, SearchResult{
			Course:     m.task.Course,
			Track:      m.task.Track,
			Topic:      m.task.Topic,
			Unit:       m.task.Unit,
			Task:       m.task.Task,
			Title:      m.task.Title,
			Tags:       m.task.Tags,
			Difficulty: m.task.Difficulty,
			Rank:       m.rank.String(),
			Snippet:    m.snippet,
		})
	}
	return results
}

// matchTask checks a single task against all query terms (AND).
func matchTask(t SearchTask, terms []string, contentOf func(SearchTask) (string, bool)) (SearchRank, string, bool) {
	// Searchable metadata: task title, unit/topic/track/course titles (packed
	// into the path context), and tags. Each term must match at least one field.
	pathText := strings.ToLower(t.Unit + " " + t.Topic + " " + t.Track + " " + t.Course)
	best := SearchRank(^uint(0) >> 1) // max int
	for _, term := range terms {
		termRank, found := matchTerm(t, pathText, term)
		if !found {
			// fall back to content
			if contentOf != nil {
				content, ok := contentOf(t)
				if ok && strings.Contains(strings.ToLower(content), term) {
					if SearchRank(RankContent) < best {
						best = RankContent
					}
					continue
				}
			}
			return 0, "", false
		}
		if termRank < best {
			best = termRank
		}
	}
	if best == SearchRank(^uint(0)>>1) {
		return 0, "", false
	}
	var snip string
	if best == RankContent && contentOf != nil {
		if content, ok := contentOf(t); ok {
			snip = snippet(content, terms)
		}
	}
	return best, snip, true
}

// matchTerm returns the best rank for one term across the task's metadata.
func matchTerm(t SearchTask, pathText, term string) (SearchRank, bool) {
	if hasWordBoundary(t.Title, term) {
		return RankTitleWord, true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), term) {
			return RankTag, true
		}
	}
	if strings.Contains(strings.ToLower(t.Title), term) {
		return RankTitle, true
	}
	if strings.Contains(pathText, term) {
		return RankTitle, true
	}
	return 0, false
}

// ResultTaskPath builds the frontend route fragment for a result.
func (r SearchResult) ResultTaskPath() string {
	return fmt.Sprintf("/courses/%s/tracks/%s/topics/%s/units/%s/tasks/%s",
		r.Course, r.Track, r.Topic, r.Unit, r.Task)
}
