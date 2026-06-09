package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(h *Handler) http.Handler {
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
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/statement", h.getStatement)
		r.Get("/courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/template", h.getTemplate)

		r.Get("/progress/{courseSlug}", h.getProgress)
		r.Put("/progress/{courseSlug}/tasks/{taskSlug}", h.updateProgress)

		r.Post("/run", h.postRun)

		r.Get("/runners", h.listRunners)
		r.Post("/runners", h.addRunner)
		r.Patch("/runners/{lang}", h.patchRunner)
		r.Delete("/runners/{lang}", h.deleteRunner)
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
