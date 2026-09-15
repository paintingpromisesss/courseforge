package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var errStatusNotOK = errors.New("provider returned error")

var openAIFetchTimeout = 30 * time.Second

const (
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01" // latest stable
)

type ModelItem struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
}

type cachedModels struct {
	items     []ModelItem
	expiresAt time.Time
}

var (
	modelsCacheMu sync.RWMutex
	modelsCache   = make(map[string]cachedModels)
)

type openRouterEndpointsResponse struct {
	Data struct {
		Endpoints []struct {
			Status int `json:"status"`
		} `json:"endpoints"`
	} `json:"data"`
}

func FetchOpenAIAvailableModels(ctx context.Context, baseURL, apiKey string, checkAvailability bool) ([]ModelItem, error) {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	cacheKey := fmt.Sprintf("%s::%s::avail=%t", baseURL, apiKey, checkAvailability)
	modelsCacheMu.RLock()
	if entry, ok := modelsCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		modelsCacheMu.RUnlock()
		return entry.items, nil
	}
	modelsCacheMu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, openAIFetchTimeout)
	defer cancel()

	endpoint := strings.TrimRight(baseURL, "/") + "/models"

	client := &http.Client{Timeout: openAIFetchTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "CourseForge/1.0")
	req.Header.Set("HTTP-Referer", "https://courseforge.dev")
	req.Header.Set("X-Title", "CourseForge")

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

	var items []ModelItem
	if checkAvailability && strings.Contains(baseURL, "openrouter.ai") {
		items = fetchOpenRouterModelsWithAvailability(ctx, baseURL, apiKey, result.Data)
	} else {
		items = make([]ModelItem, 0, len(result.Data))
		for _, m := range result.Data {
			items = append(items, ModelItem{ID: m.ID, Available: true})
		}
	}

	modelsCacheMu.Lock()
	modelsCache[cacheKey] = cachedModels{
		items:     items,
		expiresAt: time.Now().Add(2 * time.Minute),
	}
	modelsCacheMu.Unlock()

	return items, nil
}

func fetchOpenRouterModelsWithAvailability(ctx context.Context, baseURL, apiKey string, rawModels []openAIModelInfo) []ModelItem {
	items := make([]ModelItem, len(rawModels))
	var wg sync.WaitGroup

	tr := &http.Transport{
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 500,
		MaxConnsPerHost:     500,
	}
	epClient := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	for i, m := range rawModels {
		wg.Add(1)
		go func(idx int, modelID string) {
			defer wg.Done()
			if ctx.Err() != nil {
				items[idx] = ModelItem{ID: modelID, Available: false}
				return
			}
			avail := checkOpenRouterModelAvailability(ctx, epClient, baseURL, modelID, apiKey)
			items[idx] = ModelItem{ID: modelID, Available: avail}
		}(i, m.ID)
	}

	wg.Wait()
	return items
}

func checkOpenRouterModelAvailability(ctx context.Context, client *http.Client, baseURL, modelID, apiKey string) bool {
	epURL := strings.TrimRight(baseURL, "/") + "/models/" + modelID + "/endpoints"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, epURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "CourseForge/1.0")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var epData openRouterEndpointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&epData); err != nil {
		return false
	}

	for _, ep := range epData.Data.Endpoints {
		if ep.Status == 0 {
			return true
		}
	}
	return false
}

// FetchAnthropicModels calls GET /v1/models to fetch all models
// available to the provided API key. Requires x-api-key + anthropic-version headers.
func FetchAnthropicModels(ctx context.Context, baseURL, apiKey string) ([]ModelItem, error) {
	ctx, cancel := context.WithTimeout(ctx, openAIFetchTimeout)
	defer cancel()

	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	var endpoint string
	if strings.HasSuffix(baseURL, "/models") {
		endpoint = baseURL
	} else if strings.HasSuffix(baseURL, "/v1") {
		endpoint = baseURL + "/models"
	} else {
		endpoint = baseURL + "/v1/models"
	}

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

	var items []ModelItem
	for _, m := range result.Data {
		items = append(items, ModelItem{ID: m.ID, Available: true})
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
			items = append(items, ModelItem{ID: m.ID, Available: true})
		}
		after = result2.LastID
		hasMore = result2.HasMore
	}

	return items, nil
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
