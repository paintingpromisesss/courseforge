package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

type RouterOptions struct {
	FrontendDir string
}

func NewRouter(h *Handler, opts RouterOptions) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/api", func(r chi.Router) {
		r.Get("/courses", h.listCourses)
		r.Get("/courses/{courseSlug}", h.getCourse)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/theory", h.getTheory)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/assets/{filename}", h.getUnitAsset)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/statement", h.getStatement)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/template", h.getTemplate)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/tests", h.getTests)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/assets/{filename}", h.getTaskAsset)

		r.Get("/progress/{courseSlug}", h.getProgress)
		r.Put("/progress/{courseSlug}/tasks/{taskSlug}", h.updateProgress)

		r.Post("/run", h.postRun)

		r.Post("/courses/upload", h.uploadCourse)
		r.Post("/courses/import", h.importCourse)

		r.Get("/runners", h.listRunners)
		r.Post("/runners", h.addRunner)
		r.Patch("/runners/{lang}", h.patchRunner)
		r.Delete("/runners/{lang}", h.deleteRunner)
		r.Post("/runners/install", h.installRunner)
		r.Get("/runners/install/{lang}/status", h.getInstallStatus)

		r.Get("/submissions", h.listSubmissions)
		r.Post("/submissions", h.createSubmission)
	})

	if opts.FrontendDir != "" {
		frontend, err := newFrontendHandler(opts.FrontendDir)
		if err != nil {
			return nil, err
		}
		r.Get("/*", frontend.ServeHTTP)
		r.Head("/*", frontend.ServeHTTP)
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

	fsys := os.DirFS(absDir)
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		return nil, fmt.Errorf("frontend build not found in %s", absDir)
	}

	files := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/swagger") {
			http.NotFound(w, r)
			return
		}

		cleanPath := path.Clean("/" + r.URL.Path)
		if cleanPath == "/" {
			http.ServeFile(w, r, filepath.Join(absDir, "index.html"))
			return
		}

		name := strings.TrimPrefix(cleanPath, "/")
		if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(absDir, "index.html"))
	}), nil
}
