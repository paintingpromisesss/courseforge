package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCoursesAndDataDir(t *testing.T) {
	coursesDir := DefaultCoursesDir()
	if coursesDir == "" {
		t.Fatalf("expected non-empty DefaultCoursesDir")
	}

	dataDir := DefaultDataDir()
	if dataDir == "" {
		t.Fatalf("expected non-empty DefaultDataDir")
	}

	t.Logf("DefaultCoursesDir: %s", coursesDir)
	t.Logf("DefaultDataDir: %s", dataDir)
}

func TestResolveDefaultDirMock(t *testing.T) {
	tempDir := t.TempDir()
	dummyCourses := filepath.Join(tempDir, "courses")
	if err := os.Mkdir(dummyCourses, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Change working directory temporarily to tempDir
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	res := resolveDefaultDir("courses")
	absExpected, _ := filepath.Abs(dummyCourses)
	if res != absExpected && res != dummyCourses {
		t.Errorf("expected %s or %s, got %s", absExpected, dummyCourses, res)
	}
}

func TestPortAndAddrDefaults(t *testing.T) {
	t.Setenv("COURSEFORGE_PORT", "")
	t.Setenv("COURSEFORGE_ADDR", "")

	if got := DefaultPortFromEnv(); got != DefaultPort {
		t.Errorf("expected default port %d, got %d", DefaultPort, got)
	}
	if got := DefaultServerURL(); got != "http://127.0.0.1:6770" {
		t.Errorf("expected http://127.0.0.1:6770, got %s", got)
	}
	if got := DefaultAddr(); got != "127.0.0.1:6770" {
		t.Errorf("expected 127.0.0.1:6770, got %s", got)
	}

	t.Setenv("COURSEFORGE_PORT", "9090")
	if got := DefaultPortFromEnv(); got != 9090 {
		t.Errorf("expected port 9090, got %d", got)
	}
	if got := DefaultServerURL(); got != "http://127.0.0.1:9090" {
		t.Errorf("expected http://127.0.0.1:9090, got %s", got)
	}
	if got := DefaultAddr(); got != "127.0.0.1:9090" {
		t.Errorf("expected 127.0.0.1:9090, got %s", got)
	}

	t.Setenv("COURSEFORGE_ADDR", "0.0.0.0:4321")
	if got := DefaultAddr(); got != "0.0.0.0:4321" {
		t.Errorf("expected 0.0.0.0:4321, got %s", got)
	}
}
