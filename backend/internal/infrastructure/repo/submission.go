package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	_ "modernc.org/sqlite"
)

// SubmissionRepository persists submissions in a SQLite database.
type SubmissionRepository struct {
	db *sql.DB
}

func NewDB(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	// busy_timeout: wait for the write lock instead of failing with SQLITE_BUSY
	// (the MCP process writes to the same file). Must come first: pragmas run in
	// order on every new conn, and journal_mode=WAL itself can hit BUSY.
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode=WAL&_pragma=foreign_keys=on")
	if err != nil {
		return nil, err
	}
	// Default is 2 idle conns: every extra concurrent query reopened the file.
	db.SetMaxIdleConns(8)

	if err := RunMigrations(path); err != nil {
		db.Close()
		return nil, err
	}
	// Convert to WAL once up front (persistent in the file): concurrent first-time
	// conversions from several conns fail with SQLITE_BUSY despite busy_timeout.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// NewSubmissionRepository opens (or creates) the SQLite file at path and migrates the schema.
func NewSubmissionRepository(db *sql.DB) *SubmissionRepository {
	return &SubmissionRepository{db: db}
}

// Close releases the database connection.
func (s *SubmissionRepository) Close() error {
	return s.db.Close()
}

// Insert saves a submission and returns the assigned ID.
func (s *SubmissionRepository) Insert(ctx context.Context, sub *domain.Submission) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO submissions
			(course_slug, task_slug, language, code, stdout, stderr,
			 exit_code, passed_tests, total_tests, duration_ms, timed_out, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		sub.CourseSlug, sub.TaskSlug, sub.Language, sub.Code,
		sub.Stdout, sub.Stderr, sub.ExitCode,
		sub.PassedTests, sub.TotalTests, sub.DurationMs,
		sub.TimedOut, sub.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// List returns submissions for a task ordered by newest first.
func (s *SubmissionRepository) List(ctx context.Context, courseSlug, taskSlug string) ([]domain.Submission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, course_slug, task_slug, language, code, stdout, stderr,
		       exit_code, passed_tests, total_tests, duration_ms, timed_out, created_at
		FROM submissions
		WHERE course_slug = ? AND task_slug = ?
		ORDER BY created_at DESC`,
		courseSlug, taskSlug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Submission
	for rows.Next() {
		var sub domain.Submission
		var timedOut int
		var createdAt string
		if err := rows.Scan(
			&sub.ID, &sub.CourseSlug, &sub.TaskSlug, &sub.Language, &sub.Code,
			&sub.Stdout, &sub.Stderr, &sub.ExitCode,
			&sub.PassedTests, &sub.TotalTests, &sub.DurationMs,
			&timedOut, &createdAt,
		); err != nil {
			return nil, err
		}
		sub.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at for submission %d: %w", sub.ID, err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
