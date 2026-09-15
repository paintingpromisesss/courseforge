package domain

// MCPConfig holds settings for the Model Context Protocol integration.
type MCPConfig struct {
	Enabled    bool   `json:"enabled"`
	Transport  string `json:"transport"` // "stdio" or "sse"
	Host       string `json:"host"`
	Port       int    `json:"port"`
	CoursesDir string `json:"courses_dir"`
	DataDir    string `json:"data_dir"`
}

// MCPStatusResponse includes configuration and runtime environment status.
type MCPStatusResponse struct {
	MCPConfig
	BinaryPath string `json:"binary_path"`
	Platform   string `json:"platform"`
	ToolsCount int    `json:"tools_count"`
	Available  bool   `json:"available"`
}
