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
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode=WAL&_pragma=foreign_keys=on")
	if err != nil {
		return nil, err
	}

	if err := RunMigrations(path); err != nil {
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

// ExportCourse returns all submissions for a course, ordered by created_at ASC.
func (s *SubmissionRepository) ExportCourse(ctx context.Context, courseSlug string) ([]domain.Submission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, course_slug, task_slug, language, code, stdout, stderr,
		       exit_code, passed_tests, total_tests, duration_ms, timed_out, created_at
		FROM submissions
		WHERE course_slug = ?
		ORDER BY created_at ASC, id ASC`,
		courseSlug,
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
		sub.TimedOut = timedOut != 0
		var parseErr error
		sub.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAt)
		if parseErr != nil {
			sub.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// ExportAll returns all submissions grouped by course_slug, ordered by created_at ASC.
func (s *SubmissionRepository) ExportAll(ctx context.Context) (map[string][]domain.Submission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, course_slug, task_slug, language, code, stdout, stderr,
		       exit_code, passed_tests, total_tests, duration_ms, timed_out, created_at
		FROM submissions
		ORDER BY course_slug ASC, created_at ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byCourse := make(map[string][]domain.Submission)
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
		sub.TimedOut = timedOut != 0
		var parseErr error
		sub.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAt)
		if parseErr != nil {
			sub.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		}
		byCourse[sub.CourseSlug] = append(byCourse[sub.CourseSlug], sub)
	}
	return byCourse, rows.Err()
}

// ImportSubmissions inserts submissions into the database, skipping duplicates that match
// (course_slug, task_slug, language, created_at).
func (s *SubmissionRepository) ImportSubmissions(ctx context.Context, subs []domain.Submission) error {
	if len(subs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	checkStmt, err := tx.PrepareContext(ctx, `
		SELECT 1 FROM submissions
		WHERE course_slug = ? AND task_slug = ? AND language = ? AND created_at = ?
		LIMIT 1`)
	if err != nil {
		return err
	}
	defer checkStmt.Close()

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO submissions
			(course_slug, task_slug, language, code, stdout, stderr,
			 exit_code, passed_tests, total_tests, duration_ms, timed_out, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	for _, sub := range subs {
		createdAtStr := sub.CreatedAt.UTC().Format(time.RFC3339)
		var exists int
		err := checkStmt.QueryRowContext(ctx, sub.CourseSlug, sub.TaskSlug, sub.Language, createdAtStr).Scan(&exists)
		if err == nil {
			// Record already exists; skip duplicate
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}

		timedOut := 0
		if sub.TimedOut {
			timedOut = 1
		}
		if _, err := insertStmt.ExecContext(ctx,
			sub.CourseSlug, sub.TaskSlug, sub.Language, sub.Code,
			sub.Stdout, sub.Stderr, sub.ExitCode,
			sub.PassedTests, sub.TotalTests, sub.DurationMs,
			timedOut, createdAtStr,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
