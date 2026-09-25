package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/config"
)

// DiscoverServerDirs asks the local CourseForge server (GET /api/info) for its dirs.
func DiscoverServerDirs(serverURL string) (coursesDir, dataDir string, ok bool) {
	if serverURL == "" {
		serverURL = config.DefaultServerURL()
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
