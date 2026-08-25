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
	err := c.ChatStream(ctx, messages, func(chunk string) {
		result.WriteString(chunk)
	})
	return result.String(), err
}

func (c *AIClient) ChatStream(ctx context.Context, messages []domain.Message, fn func(chunk string)) error {
	isAnthropic := c.isAnthropic()
	var body []byte
	var err error

	if isAnthropic {
		msgs := make([]anthMsg, 0, len(messages))
		for _, m := range messages {
			msgs = append(msgs, anthMsg{Role: m.Role, Content: m.Content})
		}

		req := anthropicRequest{
			Model:     c.config.Model,
			Messages:  msgs,
			System:    c.config.SystemPrompt,
			Stream:    true,
			MaxTokens: 4096,
		}
		body, err = json.Marshal(req)
	} else {
		msgs := make([]message, 0, len(messages)+1)
		if c.config.SystemPrompt != "" {
			msgs = append(msgs, message{Role: "system", Content: c.config.SystemPrompt})
		}
		for _, m := range messages {
			msgs = append(msgs, message{Role: m.Role, Content: m.Content})
		}

		req := openAIRequest{
			Model:    c.config.Model,
			Messages: msgs,
			Stream:   true,
		}
		body, err = json.Marshal(req)
	}

	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	var endpoint string
	if isAnthropic {
		baseURL := c.config.BaseURL
		if baseURL == "" {
			baseURL = defaultAnthropicBaseURL
		}
		endpoint = strings.TrimRight(baseURL, "/") + "/v1/messages"
	} else {
		baseURL := c.config.BaseURL
		if baseURL == "" {
			baseURL = defaultOpenAIBaseURL
		}
		endpoint = strings.TrimRight(baseURL, "/") + "/chat/completions"
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

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
		return fmt.Errorf("ai provider returned error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	// Stream the response
	decoder := bufio.NewScanner(resp.Body)
	decoder.Buffer(make([]byte, 1024*64), 1024*64)

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

		content := extractContent(chunkData, isAnthropic)
		if content == "" {
			continue
		}

		fn(content)
	}

	if err := decoder.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}

func extractContent(data map[string]interface{}, isAnthropic bool) string {
	if isAnthropic {
		// Streaming: delta.text for text deltas, delta.content for non-streaming
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				return text
			}
			if contentRaw, ok := delta["content"]; ok {
				if content, ok := contentRaw.(string); ok {
					return content
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
					return strings.Join(parts, "")
				}
			}
		}
		return ""
	}

	choices, ok := data["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return ""
	}
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return ""
	}
	content, ok := delta["content"].(string)
	if !ok {
		return ""
	}
	return content
}
