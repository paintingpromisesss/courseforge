package dto

import "github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"

type RunReq struct {
	Language   string `json:"language" example:"go"`
	Code       string `json:"code"     example:"package main\nfunc main() {}"`
	TestCode   string `json:"test_code"`
	TimeoutSec int    `json:"timeout_sec" example:"10"`
}

type RunResp struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
}

type PatchRunnerReq struct {
	RunCmd  *[]string `json:"run_cmd"`
	TestCmd *[]string `json:"test_cmd"`
}

type ProgressUpdateReq struct {
	Done bool `json:"done" example:"true"`
}

type CreateSubmissionReq struct {
	CourseSlug  string `json:"course_slug"`
	TaskSlug    string `json:"task_slug"`
	Language    string `json:"language"`
	Code        string `json:"code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exit_code"`
	PassedTests int    `json:"passed_tests"`
	TotalTests  int    `json:"total_tests"`
	DurationMs  int64  `json:"duration_ms"`
	TimedOut    bool   `json:"timed_out"`
}

type CreateCatalogReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type PatchCatalogReq struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Courses     *[]string `json:"courses"`
}

type RunnerDriver struct {
	RunCmd    []string          `json:"run_cmd"`
	TestCmd   []string          `json:"test_cmd"`
	Ext       string            `json:"ext"`
	TestExt   string            `json:"test_ext"`
	InitFiles map[string]string `json:"init_files,omitempty"`
}

func ToRunnerDriver(d runner.LangDriver) RunnerDriver {
	return RunnerDriver{
		RunCmd:    append([]string(nil), d.RunCmd...),
		TestCmd:   append([]string(nil), d.TestCmd...),
		Ext:       d.Ext,
		TestExt:   d.TestExt,
		InitFiles: cloneStringMap(d.InitFiles),
	}
}

func ToRunnerDrivers(drivers map[string]runner.LangDriver) map[string]RunnerDriver {
	out := make(map[string]RunnerDriver, len(drivers))
	for lang, driver := range drivers {
		out[lang] = ToRunnerDriver(driver)
	}
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}
