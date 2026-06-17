package repo

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrationsWithFilePath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "courseforge.db")

	if err := RunMigrations(dbPath); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var name string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'submissions'").Scan(&name); err != nil {
		t.Fatalf("submissions table was not created: %v", err)
	}
}
