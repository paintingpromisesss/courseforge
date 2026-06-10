package di

import (
	"os"
	"path/filepath"
)

type Config struct {
	DataDir     string
	CoursesDir  string
	RunnersDir  string
	RunnersJSON string
	FrontendDir string
	Addr        string
	DBPath      string
}

func Load() *Config {
	dataDir := getenv("COURSEFORGE_DATA_DIR", "./data")
	return &Config{
		DataDir:     dataDir,
		CoursesDir:  getenv("COURSEFORGE_COURSES_DIR", "./courses"),
		RunnersDir:  filepath.Join(dataDir, "runners", "bin"),
		RunnersJSON: filepath.Join(dataDir, "runners.json"),
		FrontendDir: getenv("COURSEFORGE_FRONTEND_DIR", ""),
		Addr:        getenv("COURSEFORGE_ADDR", ":8080"),
		DBPath:      getenv("COURSEFORGE_DB_PATH", filepath.Join(dataDir, "courseforge.db")),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
