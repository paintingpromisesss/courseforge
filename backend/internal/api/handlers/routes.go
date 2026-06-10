package handlers

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(r chi.Router) {
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
}
