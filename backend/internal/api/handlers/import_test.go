package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

// moveDir falls back to os.CopyFS across filesystems; verify it reproduces a
// nested tree with file contents intact.
func TestMoveDir_NestedTree(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")

	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "top.txt"), []byte("top"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "deep.txt"), []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := moveDir(src, dst); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join(dst, "top.txt"):            "top",
		filepath.Join(dst, "a", "b", "deep.txt"): "deep",
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestMoveDir_RemovesSource(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	dst := filepath.Join(base, "dst")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := moveDir(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be gone after move")
	}
	if _, err := os.Stat(filepath.Join(dst, "f.txt")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}
}
