package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type ClientConfig struct {
	Provider     domain.Provider
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	Timeout      time.Duration
}

type AIClient struct {
	client *http.Client
	config ClientConfig
}

func NewClient(cfg ClientConfig) *AIClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &AIClient{
		client: &http.Client{},
		config: cfg,
	}
}

func (c *AIClient) isAnthropic() bool {
	return c.config.Provider == domain.ProviderAnthropic
}

func (c *AIClient) Config() ClientConfig {
	return c.config
}

func (c *AIClient) Chat(ctx context.Context, messages []domain.Message) (string, error) {
	var result strings.Builder
	err := c.ChatStream(ctx, messages, nil, func(chunk string) {
		if strings.HasPrefix(chunk, "<!-- CF_") {
			return
		}
		result.WriteString(chunk)
	})
	return result.String(), err
}

func (c *AIClient) ChatStream(ctx context.Context, messages []domain.Message, thinking *bool, fn func(chunk string)) error {
	isAnthropic := c.isAnthropic()
	var body []byte
	var err error

	var systemParts []string
	if strings.TrimSpace(c.config.SystemPrompt) != "" {
		systemParts = append(systemParts, strings.TrimSpace(c.config.SystemPrompt))
	}

	var nonSystemMessages []domain.Message
	for _, m := range messages {
		if m.Role == "system" {
			if strings.TrimSpace(m.Content) != "" {
				systemParts = append(systemParts, strings.TrimSpace(m.Content))
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, m)
		}
	}
	combinedSystem := strings.Join(systemParts, "\n\n")

	if isAnthropic {
		msgs := make([]anthMsg, 0, len(nonSystemMessages))
		for _, m := range nonSystemMessages {
			msgs = append(msgs, anthMsg{Role: m.Role, Content: m.Content})
		}

		req := anthropicRequest{
			Model:     c.config.Model,
			Messages:  msgs,
			System:    combinedSystem,
			Stream:    true,
			MaxTokens: 4096,
		}
		if thinking != nil && *thinking {
			req.Thinking = &anthropicThinking{
				Type:         "enabled",
				BudgetTokens: 2048,
			}
		}
		body, err = json.Marshal(req)
	} else {
		msgs := make([]message, 0, len(nonSystemMessages)+1)
		if combinedSystem != "" {
			msgs = append(msgs, message{Role: "system", Content: combinedSystem})
		}
		for _, m := range nonSystemMessages {
			msgs = append(msgs, message{Role: m.Role, Content: m.Content})
		}

		req := openAIRequest{
			Model:    c.config.Model,
			Messages: msgs,
			Stream:   true,
		}
		if thinking != nil {
			req.EnableThinking = thinking
			if !*thinking {
				req.ReasoningEffort = "none"
				req.ChatTemplateKwargs = map[string]interface{}{"reasoning": false}
			} else {
				req.ReasoningEffort = "medium"
				req.ChatTemplateKwargs = map[string]interface{}{"reasoning": true}
			}
		}
		body, err = json.Marshal(req)
	}

	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	var endpoint string
	if isAnthropic {
		baseURL := strings.TrimRight(c.config.BaseURL, "/")
		if baseURL == "" {
			baseURL = defaultAnthropicBaseURL
		}
		if strings.HasSuffix(baseURL, "/messages") {
			endpoint = baseURL
		} else if strings.HasSuffix(baseURL, "/v1") {
			endpoint = baseURL + "/messages"
		} else {
			endpoint = baseURL + "/v1/messages"
		}
	} else {
		baseURL := strings.TrimRight(c.config.BaseURL, "/")
		if baseURL == "" {
			baseURL = defaultOpenAIBaseURL
		}
		if strings.HasSuffix(baseURL, "/chat/completions") {
			endpoint = baseURL
		} else {
			endpoint = baseURL + "/chat/completions"
		}
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "CourseForge/1.0")
	req.Header.Set("HTTP-Referer", "https://courseforge.dev")
	req.Header.Set("X-Title", "CourseForge")

	if isAnthropic {
		req.Header.Set("content-type", "application/json")
		req.Header.Set("x-api-key", c.config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("content-type", "application/json")
		if c.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		lowerBody := strings.ToLower(bodyStr)
		if resp.StatusCode == http.StatusBadRequest && (strings.Contains(lowerBody, "reasoning is mandatory") || strings.Contains(lowerBody, "reasoning cannot be disabled") || (strings.Contains(lowerBody, "reasoning") && strings.Contains(lowerBody, "mandatory"))) {
			fn("<!-- CF_REASONING_MANDATORY -->")
			enableThinking := true
			return c.ChatStream(ctx, messages, &enableThinking, fn)
		}
		return fmt.Errorf("ai provider returned error: %d %s", resp.StatusCode, bodyStr)
	}

	// Stream the response
	decoder := bufio.NewScanner(resp.Body)
	decoder.Buffer(make([]byte, 1024*64), 1024*64)

	var inReasoning bool
	var reportedModel string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !decoder.Scan() {
			break
		}
		line := decoder.Text()

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))

		if data == "[DONE]" {
			break
		}

		var chunkData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunkData); err != nil {
			continue
		}

		if errVal, ok := chunkData["error"]; ok && errVal != nil {
			var errMsg string
			if errMap, ok := errVal.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok && msg != "" {
					errMsg = msg
				} else {
					b, _ := json.Marshal(errMap)
					errMsg = string(b)
				}
			} else if str, ok := errVal.(string); ok && str != "" {
				errMsg = str
			}
			if errMsg != "" {
				return fmt.Errorf("ai provider error: %s", errMsg)
			}
		}

		var currentModel string
		if m, ok := chunkData["model"].(string); ok && m != "" {
			currentModel = m
		} else if msg, ok := chunkData["message"].(map[string]interface{}); ok {
			if m, ok := msg["model"].(string); ok && m != "" {
				currentModel = m
			}
		}

		if currentModel != "" && reportedModel == "" {
			reportedModel = currentModel
			fn("<!-- CF_MODEL:" + currentModel + " -->")
		}

		chunk := extractChunk(chunkData, isAnthropic)
		if chunk.finishReason == "length" || chunk.finishReason == "max_tokens" {
			fn("<!-- CF_FINISH_LENGTH -->")
		}

		if chunk.content == "" {
			continue
		}

		if chunk.isReasoning {
			if !inReasoning {
				fn("<think>")
				inReasoning = true
			}
			fn(chunk.content)
		} else {
			if inReasoning {
				fn("</think>")
				inReasoning = false
			}
			fn(chunk.content)
		}
	}

	if reportedModel == "" && c.config.Model != "" {
		fn("<!-- CF_MODEL:" + c.config.Model + " -->")
	}

	if inReasoning {
		fn("</think>")
	}

	if err := decoder.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}

type chunkResult struct {
	content      string
	isReasoning  bool
	finishReason string
}

func extractChunk(data map[string]interface{}, isAnthropic bool) chunkResult {
	if isAnthropic {
		var finishReason string
		if stopReason, ok := data["stop_reason"].(string); ok && stopReason != "" {
			finishReason = stopReason
		}
		// Streaming: delta.text for text deltas, delta.content for non-streaming
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if stopReason, ok := delta["stop_reason"].(string); ok && stopReason != "" {
				finishReason = stopReason
			}
			if text, ok := delta["text"].(string); ok {
				return chunkResult{content: text, finishReason: finishReason}
			}
			if contentRaw, ok := delta["content"]; ok {
				if content, ok := contentRaw.(string); ok {
					return chunkResult{content: content, finishReason: finishReason}
				}
				if blocks, ok := contentRaw.([]interface{}); ok {
					var parts []string
					for _, b := range blocks {
						block, ok := b.(map[string]interface{})
						if !ok {
							continue
						}
						if blockType, _ := block["type"].(string); blockType == "text" {
							if text, ok := block["text"].(string); ok {
								parts = append(parts, text)
							}
						}
					}
					return chunkResult{content: strings.Join(parts, ""), finishReason: finishReason}
				}
			}
		}
		return chunkResult{finishReason: finishReason}
	}

	var finishReason string
	if doneReason, ok := data["done_reason"].(string); ok && doneReason != "" {
		finishReason = doneReason
	}

	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		if resp, ok := data["response"].(string); ok {
			return chunkResult{content: resp, finishReason: finishReason}
		}
		return chunkResult{finishReason: finishReason}
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return chunkResult{finishReason: finishReason}
	}
	if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
		finishReason = fr
	}
	if delta, ok := choice["delta"].(map[string]interface{}); ok {
		if content, ok := delta["content"].(string); ok && content != "" {
			return chunkResult{content: content, isReasoning: false, finishReason: finishReason}
		}
		if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
			return chunkResult{content: reasoning, isReasoning: true, finishReason: finishReason}
		}
	}
	if text, ok := choice["text"].(string); ok && text != "" {
		return chunkResult{content: text, finishReason: finishReason}
	}
	if msg, ok := choice["message"].(map[string]interface{}); ok {
		if content, ok := msg["content"].(string); ok {
			return chunkResult{content: content, finishReason: finishReason}
		}
	}
	return chunkResult{finishReason: finishReason}
}
