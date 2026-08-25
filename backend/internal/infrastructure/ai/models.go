package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var errStatusNotOK = errors.New("provider returned error")

var openAIFetchTimeout = 10 * time.Second

const (
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01" // latest stable
)

func FetchOpenAIAvailableModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, openAIFetchTimeout)
	defer cancel()

	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/models"

	client := &http.Client{Timeout: openAIFetchTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return nil, fmt.Errorf("%w: %d %s", errStatusNotOK, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%w: %d", errStatusNotOK, resp.StatusCode)
	}

	var result openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var modelIDs []string
	for _, modelInfo := range result.Data {
		modelIDs = append(modelIDs, modelInfo.ID)
	}

	return modelIDs, nil
}

// FetchAnthropicModels calls GET /v1/models to fetch all models
// available to the provided API key. Requires x-api-key + anthropic-version headers.
func FetchAnthropicModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, openAIFetchTimeout)
	defer cancel()

	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/v1/models"

	client := &http.Client{Timeout: openAIFetchTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return nil, fmt.Errorf("%w: %d %s", errStatusNotOK, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%w: %d", errStatusNotOK, resp.StatusCode)
	}

	var result anthropicModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var modelIDs []string
	for _, m := range result.Data {
		modelIDs = append(modelIDs, m.ID)
	}

	// Paginate forward — use last_id as after_id cursor.
	// (first_id is only used when paginating backward via before_id.)
	after := result.LastID
	hasMore := result.HasMore
	for hasMore && after != "" {
		req2, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			break
		}
		req2.Header.Set("x-api-key", apiKey)
		req2.Header.Set("anthropic-version", anthropicVersion)
		req2.Header.Set("content-type", "application/json")
		q := req2.URL.Query()
		q.Set("after_id", after)
		q.Set("limit", "1000")
		req2.URL.RawQuery = q.Encode()

		resp2, err := client.Do(req2)
		if err != nil {
			break
		}
		if resp2.StatusCode != http.StatusOK {
			resp2.Body.Close()
			break
		}
		var result2 anthropicModelsResponse
		if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
			resp2.Body.Close()
			break
		}
		resp2.Body.Close()

		for _, m := range result2.Data {
			modelIDs = append(modelIDs, m.ID)
		}
		after = result2.LastID
		hasMore = result2.HasMore
	}

	return modelIDs, nil
}

// anthropicModelsResponse is the response schema for GET /v1/models.
// See https://docs.anthropic.com/en/api/models.
type anthropicModelsResponse struct {
	Data     []anthropicModel `json:"data"`
	HasMore  bool             `json:"has_more"`
	FirstID  string           `json:"first_id"`
	LastID   string           `json:"last_id"`
}

type anthropicModel struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	DisplayName  string `json:"display_name"`
	CreatedAt    string `json:"created_at"`
	MaxInputTok  *int   `json:"max_input_tokens"`
	MaxTokens    *int   `json:"max_tokens"`
}
