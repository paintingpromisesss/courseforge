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
