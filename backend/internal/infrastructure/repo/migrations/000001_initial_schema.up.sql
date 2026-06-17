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
    timed_out    BOOLEAN NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_submissions_task
    ON submissions (course_slug, task_slug, created_at DESC);
