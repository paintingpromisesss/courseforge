package api

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/paintingpromisesss/courseforge/internal/api/handlers"
	"github.com/paintingpromisesss/courseforge/internal/web"
)

type RouterOptions struct {
	FrontendDir string
}

func NewRouter(h *handlers.Handler, opts RouterOptions) (http.Handler, error) {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	registerSwagger(r)
	registerPprof(r)

	r.Route("/api", func(r chi.Router) {
		h.RegisterRoutes(r)
	})

	var frontendHandler http.Handler
	if opts.FrontendDir != "" {
		fh, err := newFrontendHandler(opts.FrontendDir)
		if err != nil {
			return nil, err
		}
		frontendHandler = fh
	} else if web.HasEmbedded() {
		fh, err := newFrontendHandlerFromFS(web.Dist())
		if err != nil {
			return nil, err
		}
		frontendHandler = fh
	}

	if frontendHandler != nil {
		r.Get("/*", frontendHandler.ServeHTTP)
		r.Head("/*", frontendHandler.ServeHTTP)
	}

	return r, nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newFrontendHandler(dir string) (http.Handler, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve frontend dir: %w", err)
	}

	return newFrontendHandlerFromFS(os.DirFS(absDir))
}

func newFrontendHandlerFromFS(fsys fs.FS) (http.Handler, error) {
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		return nil, fmt.Errorf("frontend build not found: %w", err)
	}

	indexHTML, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read index.html: %w", err)
	}

	var modTime time.Time
	if info, err := fs.Stat(fsys, "index.html"); err == nil {
		modTime = info.ModTime()
	}

	files := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/swagger") || strings.HasPrefix(r.URL.Path, "/debug/pprof") {
			http.NotFound(w, r)
			return
		}

		cleanPath := path.Clean("/" + r.URL.Path)
		if cleanPath == "/" {
			http.ServeContent(w, r, "index.html", modTime, bytes.NewReader(indexHTML))
			return
		}

		name := strings.TrimPrefix(cleanPath, "/")
		if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		http.ServeContent(w, r, "index.html", modTime, bytes.NewReader(indexHTML))
	}), nil
}

