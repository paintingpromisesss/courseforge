package web

import (
	"io/fs"
	"testing"
)

func TestDist(t *testing.T) {
	d := Dist()
	if d == nil {
		t.Fatal("expected non-nil fs.FS from Dist()")
	}

	// placeholder.txt should exist
	if _, err := fs.Stat(d, "placeholder.txt"); err != nil {
		t.Fatalf("expected placeholder.txt in Dist(): %v", err)
	}
}
