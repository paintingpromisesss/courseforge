package handlers

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/courses", h.listCourses)
	r.Get("/catalogs", h.listCatalogs)
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
	r.Delete("/courses/{courseSlug}", h.deleteCourse)
	r.Post("/catalogs", h.createCatalog)
	r.Patch("/catalogs/{catalogSlug}", h.patchCatalog)
	r.Delete("/catalogs/{catalogSlug}", h.deleteCatalog)

	r.Get("/runners", h.listRunners)
	r.Patch("/runners/{lang}", h.patchRunner)
	r.Post("/runners/{lang}/detect", h.detectRunner)

	r.Get("/submissions", h.listSubmissions)
	r.Post("/submissions", h.createSubmission)
}
