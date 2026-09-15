package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

func TestAIClient_ChatStream_OpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world!"}}]}`,
			`data: [DONE]`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		Provider: domain.ProviderOpenAI,
		BaseURL:  server.URL,
		Model:    "gpt-4o",
	})

	messages := []domain.Message{
		{Role: "user", Content: "Hi"},
	}

	result, err := client.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if result != "Hello world!" {
		t.Fatalf("expected 'Hello world!', got %q", result)
	}
}

func TestAIClient_ChatStream_Anthropic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		chunks := []string{
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Cla"}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"ude"}}`,
			`data: [DONE]`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		Provider: domain.ProviderAnthropic,
		BaseURL:  server.URL,
		Model:    "claude-3-5-sonnet",
	})

	messages := []domain.Message{
		{Role: "user", Content: "Hi"},
	}

	result, err := client.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if result != "Claude" {
		t.Fatalf("expected 'Claude', got %q", result)
	}
}
