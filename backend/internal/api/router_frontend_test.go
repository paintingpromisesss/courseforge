package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestFrontendHandlerFromFS(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html":       {Data: []byte("<!doctype html><html><body>Root</body></html>")},
		"assets/style.css": {Data: []byte("body { color: red; }")},
	}

	handler, err := newFrontendHandlerFromFS(mockFS)
	if err != nil {
		t.Fatalf("failed to create frontend handler: %v", err)
	}

	t.Run("serve root index.html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if body := rec.Body.String(); body != "<!doctype html><html><body>Root</body></html>" {
			t.Fatalf("unexpected body: %s", body)
		}
	})

	t.Run("serve static asset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/style.css", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if body := rec.Body.String(); body != "body { color: red; }" {
			t.Fatalf("unexpected body: %s", body)
		}
	})

	t.Run("SPA fallback for unknown path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/catalog/courses/go-basics", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if body := rec.Body.String(); body != "<!doctype html><html><body>Root</body></html>" {
			t.Fatalf("expected index.html fallback, got %s", body)
		}
	})

	t.Run("API routes return 404 in frontend handler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/nonexistent", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for /api, got %d", rec.Code)
		}
	})
}
