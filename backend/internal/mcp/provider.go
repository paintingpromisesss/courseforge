package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/parser/course"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// CourseSummary represents high-level course information.
type CourseSummary struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Language       string `json:"language"`
	TotalTasks     int    `json:"total_tasks"`
	CompletedTasks int    `json:"completed_tasks"`
}

// TaskLimits describes execution limits.
type TaskLimits struct {
	TimeoutSec int `json:"timeout_sec,omitempty"`
	MemoryMB   int `json:"memory_mb,omitempty"`
}

// TaskDetails contains full metadata for a task.
type TaskDetails struct {
	CourseSlug   string     `json:"course_slug"`
	TaskSlug     string     `json:"task_slug"`
	Title        string     `json:"title"`
	Statement    string     `json:"statement"`
	EditorialURL string     `json:"editorial_url,omitempty"`
	Languages    []string   `json:"languages"`
	Limits       TaskLimits `json:"limits"`
	IsCompleted  bool       `json:"is_completed"`
}

// SubmissionItem represents a past submission attempt.
type SubmissionItem struct {
	ID          int64     `json:"id"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	ExitCode    int       `json:"exit_code"`
	PassedTests int       `json:"passed_tests"`
	TotalTests  int       `json:"total_tests"`
	DurationMs  int64     `json:"duration_ms"`
	TimedOut    bool      `json:"timed_out"`
	CreatedAt   time.Time `json:"created_at"`
}

// RunSolutionRequest parameters for executing a solution.
type RunSolutionRequest struct {
	CourseSlug     string `json:"course_slug"`
	TaskSlug       string `json:"task_slug"`
	Language       string `json:"language"`
	Code           string `json:"code"`
	SaveSubmission bool   `json:"save_submission"`
}

// RunSolutionResult outcome of executing a solution.
type RunSolutionResult struct {
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exit_code"`
	PassedTests  int    `json:"passed_tests"`
	TotalTests   int    `json:"total_tests"`
	DurationMs   int64  `json:"duration_ms"`
	TimedOut     bool   `json:"timed_out"`
	SubmissionID int64  `json:"submission_id,omitempty"`
}

// Provider defines operations needed by the MCP server tools and resources.
type Provider interface {
	ListCourses(ctx context.Context) ([]CourseSummary, error)
	GetTaskDetails(ctx context.Context, courseSlug, taskSlug string) (*TaskDetails, error)
	GetTaskStatement(ctx context.Context, courseSlug, taskSlug string) (string, error)
	GetTaskTemplate(ctx context.Context, courseSlug, taskSlug, language string) (filename, code string, err error)
	GetTaskSolution(ctx context.Context, courseSlug, taskSlug, language string) (filename, code string, err error)
	GetTaskTests(ctx context.Context, courseSlug, taskSlug, language string) (filename, code string, err error)
	ListSubmissions(ctx context.Context, courseSlug, taskSlug string, limit int) ([]SubmissionItem, error)
	GetLastSubmission(ctx context.Context, courseSlug, taskSlug string) (*SubmissionItem, error)
	RunSolution(ctx context.Context, req RunSolutionRequest) (*RunSolutionResult, error)
	ReloadCourses(ctx context.Context) error
}

// CourseForgeProvider adapts CourseForge core parser, runner, and repos to MCP.
type CourseForgeProvider struct {
	mu             sync.RWMutex
	coursesDir     string
	courses        map[string]*domain.Course
	catalogs       map[string]*domain.Catalog
	progressRepo   *repo.FileProgressRepository
	submissionRepo *repo.SubmissionRepository
	runner         *runner.Runner
}

// NewCourseForgeProvider creates a new CourseForgeProvider.
func NewCourseForgeProvider(
	coursesDir string,
	progressRepo *repo.FileProgressRepository,
	submissionRepo *repo.SubmissionRepository,
	r *runner.Runner,
) (*CourseForgeProvider, error) {
	courses, catalogs, err := course.LoadAll(coursesDir)
	if err != nil {
		return nil, fmt.Errorf("load courses: %w", err)
	}

	return &CourseForgeProvider{
		coursesDir:     coursesDir,
		courses:        courses,
		catalogs:       catalogs,
		progressRepo:   progressRepo,
		submissionRepo: submissionRepo,
		runner:         r,
	}, nil
}

func (p *CourseForgeProvider) ReloadCourses(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	courses, catalogs, err := course.LoadAll(p.coursesDir)
	if err != nil {
		return fmt.Errorf("reload courses: %w", err)
	}
	p.courses = courses
	p.catalogs = catalogs
	return nil
}

func (p *CourseForgeProvider) ListCourses(ctx context.Context) ([]CourseSummary, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var res []CourseSummary
	for _, c := range p.courses {
		totalTasks := 0
		completedCount := 0

		prog, _ := p.progressRepo.Load(ctx, c.Dir, c.Slug)

		for _, tr := range c.Tracks {
			for _, tp := range tr.Topics {
				for _, u := range tp.Units {
					for _, task := range u.Tasks {
						totalTasks++
						if prog != nil && prog.CompletedTasks[task.Slug] {
							completedCount++
						}
					}
				}
			}
		}

		res = append(res, CourseSummary{
			Slug:           c.Slug,
			Title:          c.Title,
			Description:    c.Description,
			Language:       c.Language,
			TotalTasks:     totalTasks,
			CompletedTasks: completedCount,
		})
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Slug < res[j].Slug
	})

	return res, nil
}

func (p *CourseForgeProvider) findTask(courseSlug, taskSlug string) (*domain.Course, *domain.Track, *domain.Topic, *domain.Unit, *domain.Task, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	c, ok := p.courses[courseSlug]
	if !ok {
		return nil, nil, nil, nil, nil, fmt.Errorf("course not found: %s", courseSlug)
	}
	tr, tp, u, task := c.FindTaskPath(taskSlug)
	if task == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("task not found: %s in course %s", taskSlug, courseSlug)
	}
	return c, tr, tp, u, task, nil
}

func (p *CourseForgeProvider) GetTaskDetails(ctx context.Context, courseSlug, taskSlug string) (*TaskDetails, error) {
	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return nil, err
	}

	statementPath := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, task.Statement)
	statementBytes, err := os.ReadFile(statementPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read task statement: %w", err)
	}

	var langs []string
	for lang := range task.Languages {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	var limits TaskLimits
	if task.Limits != nil {
		limits.TimeoutSec = task.Limits.TimeoutSec
		limits.MemoryMB = task.Limits.MemoryMB
	}

	isCompleted := false
	if p.progressRepo != nil {
		prog, _ := p.progressRepo.Load(ctx, c.Dir, c.Slug)
		if prog != nil && prog.CompletedTasks[task.Slug] {
			isCompleted = true
		}
	}

	return &TaskDetails{
		CourseSlug:   c.Slug,
		TaskSlug:     task.Slug,
		Title:        task.Title,
		Statement:    string(statementBytes),
		EditorialURL: task.EditorialURL,
		Languages:    langs,
		Limits:       limits,
		IsCompleted:  isCompleted,
	}, nil
}

func (p *CourseForgeProvider) GetTaskStatement(ctx context.Context, courseSlug, taskSlug string) (string, error) {
	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return "", err
	}
	statementPath := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, task.Statement)
	data, err := os.ReadFile(statementPath)
	if err != nil {
		return "", fmt.Errorf("failed to read statement: %w", err)
	}
	return string(data), nil
}

func (p *CourseForgeProvider) GetTaskTemplate(ctx context.Context, courseSlug, taskSlug, language string) (string, string, error) {
	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return "", "", err
	}
	langData, ok := task.Languages[language]
	if !ok {
		return "", "", fmt.Errorf("language %s not available for task %s", language, taskSlug)
	}
	if langData.Template == "" {
		return "", "", fmt.Errorf("no template file configured for language %s", language)
	}

	path := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, language, langData.Template)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to read template: %w", err)
	}
	return langData.Template, string(data), nil
}

func (p *CourseForgeProvider) GetTaskSolution(ctx context.Context, courseSlug, taskSlug, language string) (string, string, error) {
	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return "", "", err
	}
	langData, ok := task.Languages[language]
	if !ok {
		return "", "", fmt.Errorf("language %s not available for task %s", language, taskSlug)
	}
	if langData.Solution == "" {
		return "", "", fmt.Errorf("no solution file configured for language %s", language)
	}

	path := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, language, langData.Solution)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to read solution: %w", err)
	}
	return langData.Solution, string(data), nil
}

func (p *CourseForgeProvider) GetTaskTests(ctx context.Context, courseSlug, taskSlug, language string) (string, string, error) {
	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return "", "", err
	}
	langData, ok := task.Languages[language]
	if !ok {
		return "", "", fmt.Errorf("language %s not available for task %s", language, taskSlug)
	}
	if langData.Tests == "" {
		return "", "", fmt.Errorf("no tests file configured for language %s", language)
	}

	path := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, language, langData.Tests)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to read tests: %w", err)
	}
	return langData.Tests, string(data), nil
}

func (p *CourseForgeProvider) ListSubmissions(ctx context.Context, courseSlug, taskSlug string, limit int) ([]SubmissionItem, error) {
	if p.submissionRepo == nil {
		return nil, errors.New("submission repository not configured")
	}
	subs, err := p.submissionRepo.List(ctx, courseSlug, taskSlug)
	if err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}

	if limit <= 0 {
		limit = 10
	}
	if len(subs) > limit {
		subs = subs[:limit]
	}

	var res []SubmissionItem
	for _, s := range subs {
		res = append(res, SubmissionItem{
			ID:          s.ID,
			Language:    s.Language,
			Code:        s.Code,
			Stdout:      s.Stdout,
			Stderr:      s.Stderr,
			ExitCode:    s.ExitCode,
			PassedTests: s.PassedTests,
			TotalTests:  s.TotalTests,
			DurationMs:  s.DurationMs,
			TimedOut:    s.TimedOut,
			CreatedAt:   s.CreatedAt,
		})
	}
	return res, nil
}

func (p *CourseForgeProvider) GetLastSubmission(ctx context.Context, courseSlug, taskSlug string) (*SubmissionItem, error) {
	items, err := p.ListSubmissions(ctx, courseSlug, taskSlug, 1)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func (p *CourseForgeProvider) RunSolution(ctx context.Context, req RunSolutionRequest) (*RunSolutionResult, error) {
	if p.runner == nil {
		return nil, errors.New("runner not configured")
	}

	c, tr, tp, u, task, err := p.findTask(req.CourseSlug, req.TaskSlug)
	if err != nil {
		return nil, err
	}

	ld, exists := task.Languages[req.Language]
	if !exists {
		return nil, fmt.Errorf("language %s not available for task %s", req.Language, req.TaskSlug)
	}
	if ld.Tests == "" {
		return nil, fmt.Errorf("no tests configured for language %s", req.Language)
	}

	testPath := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, req.Language, ld.Tests)
	testCode, err := os.ReadFile(testPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load task tests: %w", err)
	}

	var schemaContent string
	if ld.Schema != "" {
		schemaPath := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug, req.Language, ld.Schema)
		schemaBytes, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load task schema: %w", err)
		}
		schemaContent = string(schemaBytes)
	}

	var timeout time.Duration
	if task.Limits != nil && task.Limits.TimeoutSec > 0 {
		timeout = time.Duration(task.Limits.TimeoutSec) * time.Second
	}

	res, err := p.runner.Run(ctx, runner.RunRequest{
		Language: req.Language,
		Code:     req.Code,
		TestCode: string(testCode),
		Schema:   schemaContent,
		Timeout:  timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("runner execution failed: %w", err)
	}

	passed, total := runner.CountTestResults(req.Language, res.Stdout, res.Stderr)

	var subID int64
	if req.SaveSubmission && p.submissionRepo != nil {
		sub := &domain.Submission{
			CourseSlug:  req.CourseSlug,
			TaskSlug:    req.TaskSlug,
			Language:    req.Language,
			Code:        req.Code,
			Stdout:      res.Stdout,
			Stderr:      res.Stderr,
			ExitCode:    res.ExitCode,
			PassedTests: passed,
			TotalTests:  total,
			DurationMs:  res.Duration.Milliseconds(),
			TimedOut:    res.TimedOut,
			CreatedAt:   time.Now().UTC(),
		}
		if id, err := p.submissionRepo.Insert(ctx, sub); err == nil {
			subID = id
		}
		// Also mark done if all tests passed
		if total > 0 && passed == total && res.ExitCode == 0 && p.progressRepo != nil {
			_ = p.progressRepo.MarkDone(ctx, c.Dir, c.Slug, task.Slug)
		}
	}

	return &RunSolutionResult{
		Stdout:       res.Stdout,
		Stderr:       res.Stderr,
		ExitCode:     res.ExitCode,
		PassedTests:  passed,
		TotalTests:   total,
		DurationMs:   res.Duration.Milliseconds(),
		TimedOut:     res.TimedOut,
		SubmissionID: subID,
	}, nil
}

var _ Provider = (*CourseForgeProvider)(nil)
