package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// DiscoverServerDirs asks the local CourseForge server (GET /api/info) for its dirs.
func DiscoverServerDirs(serverURL string) (coursesDir, dataDir string, ok bool) {
	if serverURL == "" {
		serverURL = "http://127.0.0.1:8080"
	}
	client := http.Client{Timeout: 500 * time.Millisecond}
	apiURL := strings.TrimRight(serverURL, "/") + "/api/info"
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()

	var info struct {
		CoursesDir string `json:"courses_dir"`
		DataDir    string `json:"data_dir"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&info) != nil {
		return "", "", false
	}
	return info.CoursesDir, info.DataDir, info.CoursesDir != "" && info.DataDir != ""
}
