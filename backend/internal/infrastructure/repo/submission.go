package repo

import (
	"database/sql"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	_ "modernc.org/sqlite"
)

// submissionRepository persists submissions in a SQLite database.
type submissionRepository struct {
	db *sql.DB
}

// New opens (or creates) the SQLite file at path and migrates the schema.
func NewSubmissionRepository(path string) (*submissionRepository, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode=WAL&_pragma=foreign_keys=on")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &submissionRepository{db: db}, nil
}

// Close releases the database connection.
func (s *submissionRepository) Close() error {
	return s.db.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS submissions (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    course_slug  TEXT    NOT NULL,
    task_slug    TEXT    NOT NULL,
    language     TEXT    NOT NULL,
    code         TEXT    NOT NULL,
    stdout       TEXT    NOT NULL DEFAULT '',
    stderr       TEXT    NOT NULL DEFAULT '',
    exit_code    INTEGER NOT NULL DEFAULT 0,
    passed_tests INTEGER NOT NULL DEFAULT 0,
    total_tests  INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    timed_out    INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_submissions_task
    ON submissions (course_slug, task_slug, created_at DESC);
`

func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

// Insert saves a submission and returns the assigned ID.
func (s *submissionRepository) Insert(sub *domain.Submission) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO submissions
			(course_slug, task_slug, language, code, stdout, stderr,
			 exit_code, passed_tests, total_tests, duration_ms, timed_out, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		sub.CourseSlug, sub.TaskSlug, sub.Language, sub.Code,
		sub.Stdout, sub.Stderr, sub.ExitCode,
		sub.PassedTests, sub.TotalTests, sub.DurationMs,
		boolToInt(sub.TimedOut), sub.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// List returns submissions for a task ordered by newest first.
func (s *submissionRepository) List(courseSlug, taskSlug string) ([]domain.Submission, error) {
	rows, err := s.db.Query(`
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
		sub.TimedOut = timedOut != 0
		sub.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
