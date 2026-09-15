package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type aiChatRequest struct {
	Messages []domain.Message `json:"messages"`
	Thinking *bool            `json:"thinking,omitempty"`
}

type aiChatResponse struct {
	Message domain.Message `json:"message"`
}

type aiModelsRequest struct {
	Provider          domain.Provider `json:"provider"`
	BaseURL           string          `json:"base_url"`
	APIKey            string          `json:"api_key"`
	CheckAvailability bool            `json:"check_availability,omitempty"`
}

func (h *Handler) getAIConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.ai.GetConfig(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		h.writeJSON(w, http.StatusOK, &domain.AIConfig{Enabled: false})
		return
	}

	res := *cfg
	res.Enabled = h.ai.IsEnabled()
	h.writeJSON(w, http.StatusOK, res)
}

func (h *Handler) patchAIConfig(w http.ResponseWriter, r *http.Request) {
	var cfg domain.AIConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	existing, _ := h.ai.GetConfig(ctx)

	if cfg.ProviderKeys == nil {
		cfg.ProviderKeys = make(map[string]string)
	}
	if existing != nil && existing.ProviderKeys != nil {
		for k, v := range existing.ProviderKeys {
			if _, exists := cfg.ProviderKeys[k]; !exists && v != "" {
				cfg.ProviderKeys[k] = v
			}
		}
	}

	if cfg.APIKey == "********" {
		if existing != nil {
			cfg.APIKey = existing.APIKey
		} else {
			cfg.APIKey = ""
		}
	}

	// Model is the only required field to enable AI
	cfg.Enabled = strings.TrimSpace(cfg.Model) != ""

	if err := h.ai.SaveConfig(ctx, &cfg); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) postAIChat(w http.ResponseWriter, r *http.Request) {
	var req aiChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !h.ai.IsEnabled() {
		h.writeError(w, http.StatusBadRequest, "ai service is not enabled")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ctx := r.Context()
	err := h.ai.ChatStream(ctx, req.Messages, req.Thinking, func(chunk string) {
		data, _ := json.Marshal(map[string]string{"delta": chunk})
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	})

	if err != nil {
		errData, _ := json.Marshal(map[string]string{"error": err.Error()})
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(errData)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	// Send done signal
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (h *Handler) postAIModels(w http.ResponseWriter, r *http.Request) {
	var req aiModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	apiKey := req.APIKey
	if apiKey == "" {
		if cfg, _ := h.ai.GetConfig(r.Context()); cfg != nil {
			if k, ok := cfg.ProviderKeys[req.BaseURL]; ok && k != "" {
				apiKey = k
			} else if cfg.APIKey != "" && strings.EqualFold(strings.TrimRight(cfg.BaseURL, "/"), strings.TrimRight(req.BaseURL, "/")) {
				apiKey = cfg.APIKey
			}
		}
	}

	models, err := h.ai.FetchAvailableModels(r.Context(), req.Provider, req.BaseURL, apiKey, req.CheckAvailability)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, models)
}
