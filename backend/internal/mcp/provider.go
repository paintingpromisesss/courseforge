package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

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
	Difficulty   int        `json:"difficulty,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	Statement    string     `json:"statement"`
	EditorialURL string     `json:"editorial_url,omitempty"`
	VideoURL     string     `json:"video_url,omitempty"`
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

// CreateTaskRequest contains parameters to scaffold a new multilingual task.
type CreateTaskRequest struct {
	CourseSlug   string   `json:"course_slug"`
	UnitSlug     string   `json:"unit_slug"`
	TaskSlug     string   `json:"task_slug"`
	Title        string   `json:"title"`
	Statement    string   `json:"statement"`
	Language     string   `json:"language"`
	TemplateCode string   `json:"template_code"`
	TestsCode    string   `json:"tests_code"`
	SolutionCode string   `json:"solution_code"`
	Difficulty   int      `json:"difficulty,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	TimeoutSec   int      `json:"timeout_sec,omitempty"`
	MemoryMB     int      `json:"memory_mb,omitempty"`
	EditorialURL string   `json:"editorial_url,omitempty"`
	VideoURL     string   `json:"video_url,omitempty"`
}

// UpdateTaskMetadataRequest contains fields to update in task.yaml.
type UpdateTaskMetadataRequest struct {
	CourseSlug   string    `json:"course_slug"`
	TaskSlug     string    `json:"task_slug"`
	Title        *string   `json:"title,omitempty"`
	Difficulty   *int      `json:"difficulty,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
	TimeoutSec   *int      `json:"timeout_sec,omitempty"`
	MemoryMB     *int      `json:"memory_mb,omitempty"`
	EditorialURL *string   `json:"editorial_url,omitempty"`
	VideoURL     *string   `json:"video_url,omitempty"`
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

	CreateTask(ctx context.Context, req CreateTaskRequest) (*TaskDetails, error)
	DeleteTask(ctx context.Context, courseSlug, taskSlug string) error
	EditTaskStatement(ctx context.Context, courseSlug, taskSlug, content string) error
	EditTaskCode(ctx context.Context, courseSlug, taskSlug, language, fileType, content string) (string, error)
	UpdateTaskMetadata(ctx context.Context, req UpdateTaskMetadataRequest) (*TaskDetails, error)
	EditUnitTheory(ctx context.Context, courseSlug, unitSlug, content string) error
	SaveNote(ctx context.Context, courseSlug, unitSlug, content, mode string) error
	GetNote(ctx context.Context, courseSlug, unitSlug string) (content string, hasNote bool, err error)
	DeleteNote(ctx context.Context, courseSlug, unitSlug string) error
}

// CourseForgeProvider adapts CourseForge core parser, runner, and repos to MCP.
type CourseForgeProvider struct {
	mu             sync.RWMutex
	coursesDir     string
	dataDir        string
	courses        map[string]*domain.Course
	catalogs       map[string]*domain.Catalog
	progressRepo   *repo.FileProgressRepository
	submissionRepo *repo.SubmissionRepository
	runner         *runner.Runner
}

// NewCourseForgeProvider creates a new CourseForgeProvider.
func NewCourseForgeProvider(
	coursesDir string,
	dataDir string,
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
		dataDir:        dataDir,
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

	editorialURL := task.VideoURL
	if editorialURL == "" {
		editorialURL = task.EditorialURL
	}

	return &TaskDetails{
		CourseSlug:   c.Slug,
		TaskSlug:     task.Slug,
		Title:        task.Title,
		Difficulty:   task.Difficulty,
		Tags:         task.Tags,
		Statement:    string(statementBytes),
		EditorialURL: editorialURL,
		VideoURL:     editorialURL,
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

var slugRegex = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

func validateSlug(slug, name string) error {
	if slug == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("invalid %s format: %q (only alphanumeric/unicode letters, digits, hyphens and underscores allowed)", name, slug)
	}
	return nil
}

func isPathUnder(targetPath, baseDir string) bool {
	cleanTarget := filepath.Clean(targetPath)
	cleanBase := filepath.Clean(baseDir)
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func defaultExtForLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "go":
		return ".go"
	case "python", "python3", "py":
		return ".py"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "cpp", "c++":
		return ".cpp"
	case "c":
		return ".c"
	case "csharp", "cs":
		return ".cs"
	case "java":
		return ".java"
	case "rust", "rs":
		return ".rs"
	case "postgres", "sql":
		return ".sql"
	case "ruby", "rb":
		return ".rb"
	case "php":
		return ".php"
	default:
		return "." + strings.ToLower(lang)
	}
}

func defaultFilenamesForLanguage(lang string) (template, solution, tests string) {
	ext := defaultExtForLanguage(lang)
	template = "template" + ext
	solution = "solution" + ext
	switch strings.ToLower(lang) {
	case "go":
		tests = "solution_test.go"
	case "python", "python3", "py":
		tests = "test_solution.py"
	default:
		tests = "tests" + ext
	}
	return template, solution, tests
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (p *CourseForgeProvider) findUnit(courseSlug, unitSlug string) (*domain.Course, *domain.Track, *domain.Topic, *domain.Unit, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	c, ok := p.courses[courseSlug]
	if !ok {
		return nil, nil, nil, nil, fmt.Errorf("course not found: %s", courseSlug)
	}
	tr, tp, u := c.FindUnitPath(unitSlug)
	if u == nil {
		return nil, nil, nil, nil, fmt.Errorf("unit not found: %s in course %s", unitSlug, courseSlug)
	}
	return c, tr, tp, u, nil
}

func (p *CourseForgeProvider) CreateTask(ctx context.Context, req CreateTaskRequest) (*TaskDetails, error) {
	if err := validateSlug(req.CourseSlug, "course_slug"); err != nil {
		return nil, err
	}
	if err := validateSlug(req.UnitSlug, "unit_slug"); err != nil {
		return nil, err
	}
	if err := validateSlug(req.TaskSlug, "task_slug"); err != nil {
		return nil, err
	}
	if err := validateSlug(req.Language, "language"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("title cannot be empty")
	}
	if strings.TrimSpace(req.Statement) == "" {
		return nil, errors.New("statement cannot be empty")
	}
	if strings.TrimSpace(req.TemplateCode) == "" {
		return nil, errors.New("template_code cannot be empty")
	}
	if strings.TrimSpace(req.TestsCode) == "" {
		return nil, errors.New("tests_code cannot be empty")
	}
	if strings.TrimSpace(req.SolutionCode) == "" {
		return nil, errors.New("solution_code cannot be empty")
	}

	c, tr, tp, u, err := p.findUnit(req.CourseSlug, req.UnitSlug)
	if err != nil {
		return nil, err
	}

	unitDir := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug)
	taskDir := filepath.Join(unitDir, req.TaskSlug)
	if !isPathUnder(taskDir, p.coursesDir) {
		return nil, errors.New("invalid path traversal target")
	}

	if _, err := os.Stat(taskDir); err == nil {
		return nil, fmt.Errorf("task directory already exists: %s", req.TaskSlug)
	}

	langDir := filepath.Join(taskDir, req.Language)
	if err := os.MkdirAll(langDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create task language directory: %w", err)
	}

	stmtPath := filepath.Join(taskDir, "statement.md")
	if err := atomicWriteFile(stmtPath, []byte(req.Statement), 0644); err != nil {
		return nil, fmt.Errorf("failed to write statement.md: %w", err)
	}

	tmplFile, solFile, testsFile := defaultFilenamesForLanguage(req.Language)
	if err := atomicWriteFile(filepath.Join(langDir, tmplFile), []byte(req.TemplateCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write template: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(langDir, solFile), []byte(req.SolutionCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write solution: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(langDir, testsFile), []byte(req.TestsCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write tests: %w", err)
	}

	taskYAML := map[string]any{
		"slug":      req.TaskSlug,
		"title":     req.Title,
		"statement": "statement.md",
		"languages": map[string]any{
			req.Language: map[string]any{
				"template": tmplFile,
				"solution": solFile,
				"tests":    testsFile,
			},
		},
	}
	if req.Difficulty > 0 {
		taskYAML["difficulty"] = req.Difficulty
	}
	if len(req.Tags) > 0 {
		taskYAML["tags"] = req.Tags
	}
	if req.EditorialURL != "" {
		taskYAML["editorial_url"] = req.EditorialURL
	}
	if req.VideoURL != "" {
		taskYAML["video_url"] = req.VideoURL
	}
	if req.TimeoutSec > 0 || req.MemoryMB > 0 {
		lims := map[string]any{}
		if req.TimeoutSec > 0 {
			lims["timeout_sec"] = req.TimeoutSec
		}
		if req.MemoryMB > 0 {
			lims["memory_mb"] = req.MemoryMB
		}
		taskYAML["limits"] = lims
	}

	taskBytes, err := yaml.Marshal(taskYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task.yaml: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(taskDir, "task.yaml"), taskBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write task.yaml: %w", err)
	}

	unitYAMLPath := filepath.Join(unitDir, "unit.yaml")
	unitRaw, err := os.ReadFile(unitYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("read unit.yaml: %w", err)
	}
	var unitMap map[string]any
	if err := yaml.Unmarshal(unitRaw, &unitMap); err != nil {
		return nil, fmt.Errorf("parse unit.yaml: %w", err)
	}

	var taskList []any
	if existing, ok := unitMap["tasks"].([]any); ok {
		taskList = existing
	}
	alreadyListed := false
	for _, item := range taskList {
		if s, ok := item.(string); ok && s == req.TaskSlug {
			alreadyListed = true
			break
		}
	}
	if !alreadyListed {
		unitMap["tasks"] = append(taskList, req.TaskSlug)
		unitBytes, err := yaml.Marshal(unitMap)
		if err != nil {
			return nil, fmt.Errorf("marshal unit.yaml: %w", err)
		}
		if err := atomicWriteFile(unitYAMLPath, unitBytes, 0644); err != nil {
			return nil, fmt.Errorf("write unit.yaml: %w", err)
		}
	}

	if err := p.ReloadCourses(ctx); err != nil {
		return nil, fmt.Errorf("reload courses after create_task: %w", err)
	}

	return p.GetTaskDetails(ctx, req.CourseSlug, req.TaskSlug)
}

func (p *CourseForgeProvider) EditTaskStatement(ctx context.Context, courseSlug, taskSlug, content string) error {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return err
	}
	if err := validateSlug(taskSlug, "task_slug"); err != nil {
		return err
	}

	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return err
	}

	taskDir := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug)
	stmtFile := task.Statement
	if stmtFile == "" {
		stmtFile = "statement.md"
	}
	targetPath := filepath.Join(taskDir, stmtFile)
	if !isPathUnder(targetPath, p.coursesDir) {
		return errors.New("invalid path traversal target")
	}

	if err := atomicWriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write statement: %w", err)
	}

	return p.ReloadCourses(ctx)
}

func (p *CourseForgeProvider) EditTaskCode(ctx context.Context, courseSlug, taskSlug, language, fileType, content string) (string, error) {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return "", err
	}
	if err := validateSlug(taskSlug, "task_slug"); err != nil {
		return "", err
	}
	if err := validateSlug(language, "language"); err != nil {
		return "", err
	}
	switch fileType {
	case "template", "tests", "solution":
	default:
		return "", fmt.Errorf("invalid file_type: %q, must be one of 'template', 'tests', 'solution'", fileType)
	}

	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return "", err
	}

	taskDir := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug)
	langDir := filepath.Join(taskDir, language)

	tmplFile, solFile, testsFile := defaultFilenamesForLanguage(language)
	langCfg, exists := task.Languages[language]
	if exists {
		if langCfg.Template != "" {
			tmplFile = langCfg.Template
		}
		if langCfg.Solution != "" {
			solFile = langCfg.Solution
		}
		if langCfg.Tests != "" {
			testsFile = langCfg.Tests
		}
	} else {
		if err := os.MkdirAll(langDir, 0755); err != nil {
			return "", fmt.Errorf("create language directory: %w", err)
		}
		taskYAMLPath := filepath.Join(taskDir, "task.yaml")
		raw, err := os.ReadFile(taskYAMLPath)
		if err != nil {
			return "", fmt.Errorf("read task.yaml: %w", err)
		}
		var m map[string]any
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return "", fmt.Errorf("parse task.yaml: %w", err)
		}
		langs, _ := m["languages"].(map[string]any)
		if langs == nil {
			langs = make(map[string]any)
		}
		langs[language] = map[string]any{
			"template": tmplFile,
			"solution": solFile,
			"tests":    testsFile,
		}
		m["languages"] = langs
		out, err := yaml.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("marshal task.yaml: %w", err)
		}
		if err := atomicWriteFile(taskYAMLPath, out, 0644); err != nil {
			return "", fmt.Errorf("write task.yaml: %w", err)
		}
	}

	var targetFilename string
	switch fileType {
	case "template":
		targetFilename = tmplFile
	case "solution":
		targetFilename = solFile
	case "tests":
		targetFilename = testsFile
	}

	targetPath := filepath.Join(langDir, targetFilename)
	if !isPathUnder(targetPath, p.coursesDir) {
		return "", errors.New("invalid path traversal target")
	}

	if err := atomicWriteFile(targetPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", targetFilename, err)
	}

	if err := p.ReloadCourses(ctx); err != nil {
		return "", fmt.Errorf("reload courses: %w", err)
	}

	return targetFilename, nil
}

func (p *CourseForgeProvider) UpdateTaskMetadata(ctx context.Context, req UpdateTaskMetadataRequest) (*TaskDetails, error) {
	if err := validateSlug(req.CourseSlug, "course_slug"); err != nil {
		return nil, err
	}
	if err := validateSlug(req.TaskSlug, "task_slug"); err != nil {
		return nil, err
	}
	if req.Difficulty != nil && (*req.Difficulty < 1 || *req.Difficulty > 5) {
		return nil, fmt.Errorf("difficulty must be between 1 and 5, got %d", *req.Difficulty)
	}

	c, tr, tp, u, task, err := p.findTask(req.CourseSlug, req.TaskSlug)
	if err != nil {
		return nil, err
	}

	taskDir := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug, task.Slug)
	taskYAMLPath := filepath.Join(taskDir, "task.yaml")
	raw, err := os.ReadFile(taskYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("read task.yaml: %w", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse task.yaml: %w", err)
	}

	if req.Title != nil {
		m["title"] = *req.Title
	}
	if req.Difficulty != nil {
		m["difficulty"] = *req.Difficulty
	}
	if req.Tags != nil {
		m["tags"] = *req.Tags
	}
	if req.EditorialURL != nil {
		m["editorial_url"] = *req.EditorialURL
	}
	if req.VideoURL != nil {
		m["video_url"] = *req.VideoURL
	}
	if req.TimeoutSec != nil || req.MemoryMB != nil {
		limits, _ := m["limits"].(map[string]any)
		if limits == nil {
			limits = make(map[string]any)
		}
		if req.TimeoutSec != nil {
			limits["timeout_sec"] = *req.TimeoutSec
		}
		if req.MemoryMB != nil {
			limits["memory_mb"] = *req.MemoryMB
		}
		m["limits"] = limits
	}

	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal task.yaml: %w", err)
	}
	if err := atomicWriteFile(taskYAMLPath, out, 0644); err != nil {
		return nil, fmt.Errorf("write task.yaml: %w", err)
	}

	if err := p.ReloadCourses(ctx); err != nil {
		return nil, fmt.Errorf("reload courses: %w", err)
	}

	return p.GetTaskDetails(ctx, req.CourseSlug, req.TaskSlug)
}

func (p *CourseForgeProvider) EditUnitTheory(ctx context.Context, courseSlug, unitSlug, content string) error {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return err
	}
	if err := validateSlug(unitSlug, "unit_slug"); err != nil {
		return err
	}

	c, tr, tp, u, err := p.findUnit(courseSlug, unitSlug)
	if err != nil {
		return err
	}

	unitDir := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug)
	theoryFile := u.Theory
	if theoryFile == "" {
		theoryFile = "theory.md"
		unitYAMLPath := filepath.Join(unitDir, "unit.yaml")
		raw, err := os.ReadFile(unitYAMLPath)
		if err != nil {
			return fmt.Errorf("read unit.yaml: %w", err)
		}
		var m map[string]any
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("parse unit.yaml: %w", err)
		}
		m["theory"] = "theory.md"
		out, err := yaml.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal unit.yaml: %w", err)
		}
		if err := atomicWriteFile(unitYAMLPath, out, 0644); err != nil {
			return fmt.Errorf("write unit.yaml: %w", err)
		}
	}

	targetPath := filepath.Join(unitDir, theoryFile)
	if !isPathUnder(targetPath, p.coursesDir) {
		return errors.New("invalid path traversal target")
	}

	if err := atomicWriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write theory: %w", err)
	}

	return p.ReloadCourses(ctx)
}

func (p *CourseForgeProvider) SaveNote(ctx context.Context, courseSlug, unitSlug, content, mode string) error {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return err
	}
	if err := validateSlug(unitSlug, "unit_slug"); err != nil {
		return err
	}
	switch mode {
	case "overwrite", "append":
	default:
		return fmt.Errorf("invalid mode: %q, must be 'overwrite' or 'append'", mode)
	}
	if p.dataDir == "" {
		return errors.New("data directory is not configured")
	}

	notesBase := filepath.Join(p.dataDir, "notes")
	noteDir := filepath.Join(notesBase, courseSlug)
	notePath := filepath.Join(noteDir, unitSlug+".md")
	if !isPathUnder(notePath, notesBase) {
		return errors.New("invalid path traversal target")
	}

	if err := os.MkdirAll(noteDir, 0755); err != nil {
		return fmt.Errorf("create notes dir: %w", err)
	}

	finalContent := content
	if mode == "append" {
		if existing, err := os.ReadFile(notePath); err == nil && len(existing) > 0 {
			finalContent = string(existing) + "\n\n" + content
		}
	}

	return atomicWriteFile(notePath, []byte(finalContent), 0644)
}

func (p *CourseForgeProvider) GetNote(ctx context.Context, courseSlug, unitSlug string) (string, bool, error) {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return "", false, err
	}
	if err := validateSlug(unitSlug, "unit_slug"); err != nil {
		return "", false, err
	}
	if p.dataDir == "" {
		return "", false, errors.New("data directory is not configured")
	}

	notesBase := filepath.Join(p.dataDir, "notes")
	notePath := filepath.Join(notesBase, courseSlug, unitSlug+".md")
	if !isPathUnder(notePath, notesBase) {
		return "", false, errors.New("invalid path traversal target")
	}

	data, err := os.ReadFile(notePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read note: %w", err)
	}
	return string(data), true, nil
}

func (p *CourseForgeProvider) DeleteTask(ctx context.Context, courseSlug, taskSlug string) error {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return err
	}
	if err := validateSlug(taskSlug, "task_slug"); err != nil {
		return err
	}

	c, tr, tp, u, task, err := p.findTask(courseSlug, taskSlug)
	if err != nil {
		return err
	}

	unitDir := filepath.Join(p.coursesDir, c.Dir, tr.Slug, tp.Slug, u.Slug)
	taskDir := filepath.Join(unitDir, task.Slug)
	if !isPathUnder(taskDir, p.coursesDir) {
		return errors.New("invalid path traversal target")
	}

	if err := os.RemoveAll(taskDir); err != nil {
		return fmt.Errorf("remove task directory: %w", err)
	}

	unitYAMLPath := filepath.Join(unitDir, "unit.yaml")
	unitRaw, err := os.ReadFile(unitYAMLPath)
	if err == nil {
		var unitMap map[string]any
		if err := yaml.Unmarshal(unitRaw, &unitMap); err == nil {
			if existing, ok := unitMap["tasks"].([]any); ok {
				var newTasks []any
				for _, t := range existing {
					if s, ok := t.(string); ok && s == taskSlug {
						continue
					}
					newTasks = append(newTasks, t)
				}
				unitMap["tasks"] = newTasks
				if unitBytes, err := yaml.Marshal(unitMap); err == nil {
					_ = atomicWriteFile(unitYAMLPath, unitBytes, 0644)
				}
			}
		}
	}

	return p.ReloadCourses(ctx)
}

func (p *CourseForgeProvider) DeleteNote(ctx context.Context, courseSlug, unitSlug string) error {
	if err := validateSlug(courseSlug, "course_slug"); err != nil {
		return err
	}
	if err := validateSlug(unitSlug, "unit_slug"); err != nil {
		return err
	}
	if p.dataDir == "" {
		return errors.New("data directory is not configured")
	}

	notesBase := filepath.Join(p.dataDir, "notes")
	notePath := filepath.Join(notesBase, courseSlug, unitSlug+".md")
	if !isPathUnder(notePath, notesBase) {
		return errors.New("invalid path traversal target")
	}

	if err := os.Remove(notePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove note: %w", err)
	}
	return nil
}

var _ Provider = (*CourseForgeProvider)(nil)
