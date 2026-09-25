package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultPort = 6770
	DefaultHost = "127.0.0.1"
)

// DefaultPortFromEnv returns COURSEFORGE_PORT as int if valid, otherwise DefaultPort.
func DefaultPortFromEnv() int {
	if p := os.Getenv("COURSEFORGE_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= 65535 {
			return v
		}
	}
	return DefaultPort
}

// DefaultServerURL returns the default CourseForge HTTP URL (e.g. "http://127.0.0.1:6770").
func DefaultServerURL() string {
	return fmt.Sprintf("http://%s:%d", DefaultHost, DefaultPortFromEnv())
}

// DefaultAddr returns the default address string (e.g. "127.0.0.1:6770").
// It respects COURSEFORGE_ADDR if set; otherwise combines DefaultHost and DefaultPortFromEnv().
func DefaultAddr() string {
	if addr := os.Getenv("COURSEFORGE_ADDR"); addr != "" {
		return addr
	}
	return fmt.Sprintf("%s:%d", DefaultHost, DefaultPortFromEnv())
}

type Config struct {
	Version     string
	DataDir     string
	CoursesDir  string
	RunnersJSON string
	FrontendDir string
	Addr        string
	DBPath      string
	EnableTray  bool
}

func Load() *Config {
	dataDir := getenv("COURSEFORGE_DATA_DIR", DefaultDataDir())
	return &Config{
		DataDir:     dataDir,
		CoursesDir:  getenv("COURSEFORGE_COURSES_DIR", DefaultCoursesDir()),
		RunnersJSON: DefaultRunnersJSON(dataDir),
		FrontendDir: getenv("COURSEFORGE_FRONTEND_DIR", ""),
		Addr:        DefaultAddr(),
		DBPath:      getenv("COURSEFORGE_DB_PATH", DefaultDBPath(dataDir)),
	}
}

// DefaultCoursesDir resolves the default directory for course manifests.
// It prioritizes:
// 1. Directory relative to binary location (exeDir/courses or exeDir/../courses)
// 2. Existing ./courses in current working directory (or ../courses)
// 3. Absolute path to ./courses
func DefaultCoursesDir() string {
	return resolveDefaultDir("courses")
}

// DefaultDataDir resolves the default directory for application runtime state.
// It prioritizes:
// 1. Directory relative to binary location (exeDir/data or exeDir/../data)
// 2. Existing ./data in current working directory (or ../data)
// 3. Absolute path to ./data
func DefaultDataDir() string {
	return resolveDefaultDir("data")
}

func resolveDefaultDir(name string) string {
	// 1. Check relative to executable location (so the binary finds its courses/data anywhere)
	if exe, err := os.Executable(); err == nil {
		if realExe, err := filepath.EvalSymlinks(exe); err == nil {
			exe = realExe
		}
		exeDir := filepath.Dir(exe)

		isTemp := strings.Contains(exe, "go-build") ||
			strings.Contains(exe, "\\Temp\\") ||
			strings.Contains(exe, "/tmp/")

		if !isTemp {
			// Candidate A: directly next to binary (e.g. bin/courses or portable-folder/courses)
			candA := filepath.Join(exeDir, name)
			if fi, err := os.Stat(candA); err == nil && fi.IsDir() {
				if abs, err := filepath.Abs(candA); err == nil {
					return abs
				}
				return candA
			}

			// Candidate B: one level up from binary (e.g. bin/../courses -> repo/courses)
			candB := filepath.Join(exeDir, "..", name)
			if fi, err := os.Stat(candB); err == nil && fi.IsDir() {
				if abs, err := filepath.Abs(candB); err == nil {
					return abs
				}
				return candB
			}

			// If courses exists at exeDir/.. (e.g. bin/../courses), then name (e.g. data) should also default to exeDir/..
			coursesAtParent := filepath.Join(exeDir, "..", "courses")
			if fi, err := os.Stat(coursesAtParent); err == nil && fi.IsDir() {
				if abs, err := filepath.Abs(candB); err == nil {
					return abs
				}
				return candB
			}

			// If courses exists at exeDir (e.g. portable-folder/courses), then name should default to exeDir/name
			coursesAtExe := filepath.Join(exeDir, "courses")
			if fi, err := os.Stat(coursesAtExe); err == nil && fi.IsDir() {
				if abs, err := filepath.Abs(candA); err == nil {
					return abs
				}
				return candA
			}
		}
	}

	// 2. Check current working directory ./name
	if fi, err := os.Stat(name); err == nil && fi.IsDir() {
		if abs, err := filepath.Abs(name); err == nil {
			return abs
		}
		return name
	}

	// 3. Check parent of current working directory ../name
	parentCand := filepath.Join("..", name)
	if fi, err := os.Stat(parentCand); err == nil && fi.IsDir() {
		if abs, err := filepath.Abs(parentCand); err == nil {
			return abs
		}
		return parentCand
	}

	// 4. If exeDir is valid and non-temp, fallback to exeDir/name
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		isTemp := strings.Contains(exe, "go-build") ||
			strings.Contains(exe, "\\Temp\\") ||
			strings.Contains(exe, "/tmp/")
		if !isTemp {
			candA := filepath.Join(exeDir, name)
			if abs, err := filepath.Abs(candA); err == nil {
				return abs
			}
			return candA
		}
	}

	// 5. Default fallback
	if abs, err := filepath.Abs(name); err == nil {
		return abs
	}
	return "./" + name
}

func DefaultRunnersJSON(dataDir string) string {
	return filepath.Join(dataDir, "runners.json")
}

func DefaultDBPath(dataDir string) string {
	return filepath.Join(dataDir, "courseforge.db")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
