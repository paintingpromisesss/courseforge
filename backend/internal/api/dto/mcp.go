package dto

type MCPConfigReq struct {
	Enabled    *bool   `json:"enabled,omitempty"`
	Transport  *string `json:"transport,omitempty"`
	Host       *string `json:"host,omitempty"`
	Port       *int    `json:"port,omitempty"`
	CoursesDir *string `json:"courses_dir,omitempty"`
	DataDir    *string `json:"data_dir,omitempty"`
}

type MCPStatusResp struct {
	Enabled    bool     `json:"enabled"`
	Transport  string   `json:"transport"`
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	CoursesDir string   `json:"courses_dir"`
	DataDir    string   `json:"data_dir"`
	BinaryPath string   `json:"binary_path"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	SSEURL     string   `json:"sse_url"`
	Platform   string   `json:"platform"`
	ToolsCount int      `json:"tools_count"`
	Available  bool     `json:"available"`
}

type MCPActiveTaskReq struct {
	CourseSlug string `json:"course_slug"`
	TaskSlug   string `json:"task_slug"`
	Language   string `json:"language,omitempty"`
}

type MCPActiveTaskResp struct {
	CourseSlug string `json:"course_slug,omitempty"`
	TaskSlug   string `json:"task_slug,omitempty"`
	Language   string `json:"language,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

