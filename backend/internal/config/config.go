package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DataDir     string
	CoursesDir  string
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
		RunnersJSON: DefaultRunnersJSON(dataDir),
		FrontendDir: getenv("COURSEFORGE_FRONTEND_DIR", ""),
		Addr:        getenv("COURSEFORGE_ADDR", ":8080"),
		DBPath:      getenv("COURSEFORGE_DB_PATH", DefaultDBPath(dataDir)),
	}
}

// DefaultRunnersJSON returns the default runners config path under dataDir.
func DefaultRunnersJSON(dataDir string) string {
	return filepath.Join(dataDir, "runners.json")
}

// DefaultDBPath returns the default submissions database path under dataDir.
func DefaultDBPath(dataDir string) string {
	return filepath.Join(dataDir, "courseforge.db")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
