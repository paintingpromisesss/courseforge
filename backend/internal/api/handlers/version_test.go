package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/updater"
)

func TestVersionHandlers(t *testing.T) {
	tempDir := t.TempDir()
	u := updater.New("v1.0.0", tempDir)
	h := New(filepath.Join(tempDir, "courses"), tempDir, nil, nil, nil, nil, nil, nil, nil, nil, u)

	// 1. GET /api/version
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	h.getVersion(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp VersionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", resp.Version)
	}
	if resp.IsDev {
		t.Errorf("expected is_dev false for v1.0.0, got %v", resp.IsDev)
	}
	if resp.Status.State != "idle" {
		t.Errorf("expected state idle, got %s", resp.Status.State)
	}
}
