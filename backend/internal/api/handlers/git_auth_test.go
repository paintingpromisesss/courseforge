package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func TestGitAuthHandlers(t *testing.T) {
	tempDir := t.TempDir()
	gitAuth := repo.NewGitAuthRepository(tempDir)
	gitSvc := git.NewService()
	h := New(filepath.Join(tempDir, "courses"), tempDir, nil, nil, nil, nil, nil, nil, nil, gitAuth, gitSvc, nil, nil, nil)

	// 1. GET before configuration -> not configured.
	req := httptest.NewRequest(http.MethodGet, "/git/auth", nil)
	w := httptest.NewRecorder()
	h.getGitAuth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", w.Code)
	}
	var status dto.GitAuthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Configured {
		t.Fatal("expected configured=false before any token is saved")
	}

	// 2. PATCH with a token -> saved, status masked, raw token never returned.
	body := `{"token":"ghp_abcdefghijklmnop","username":"octocat"}`
	req = httptest.NewRequest(http.MethodPatch, "/git/auth", strings.NewReader(body))
	w = httptest.NewRecorder()
	h.patchGitAuth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !status.Configured || status.Username != "octocat" {
		t.Fatalf("status = %+v, want configured with username octocat", status)
	}
	if status.TokenMasked != "ghp_…mnop" {
		t.Fatalf("token_masked = %q, want ghp_…mnop", status.TokenMasked)
	}
	if strings.Contains(w.Body.String(), "ghp_abcdefghijklmnop") {
		t.Fatal("raw token leaked in PATCH response")
	}
	if gitSvc.Token() != "ghp_abcdefghijklmnop" {
		t.Fatal("token was not propagated to the git service")
	}

	req = httptest.NewRequest(http.MethodGet, "/git/auth", nil)
	w = httptest.NewRecorder()
	h.getGitAuth(w, req)
	if strings.Contains(w.Body.String(), "ghp_abcdefghijklmnop") {
		t.Fatal("raw token leaked in GET response")
	}

	// 3. PATCH with an empty token -> clears stored auth.
	req = httptest.NewRequest(http.MethodPatch, "/git/auth", strings.NewReader(`{"token":""}`))
	w = httptest.NewRecorder()
	h.patchGitAuth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH(clear) status = %d, want 200", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Configured {
		t.Fatal("expected configured=false after clearing the token")
	}
	if gitSvc.Token() != "" {
		t.Fatal("token was not cleared in the git service")
	}
	saved, err := gitAuth.Load(t.Context())
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if saved != nil {
		t.Fatal("auth file still present after clearing")
	}
}
