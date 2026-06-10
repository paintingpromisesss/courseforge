package domain

type Progress struct {
	CourseSlug     string          `json:"course_slug"`
	CompletedTasks map[string]bool `json:"completed_tasks"`
}
