-- migrations/0004_status_review.sql
-- Extend tasks.status CHECK constraint with needs_review, approved, rejected
-- for M3's diff-scan + approval flow. SQLite cannot ALTER CHECK, so we
-- recreate the table preserving all data.

-- +goose Up
CREATE TABLE tasks_new (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    description     TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN
        ('queued','running','completed','failed','cancelled','needs_review','approved','rejected')),
    branch_name     TEXT,
    summary         TEXT,
    error           TEXT,
    tokens_used     INTEGER NOT NULL DEFAULT 0,
    cost_cents      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME,
    finished_at     DATETIME
);
INSERT INTO tasks_new SELECT * FROM tasks;
DROP TABLE tasks;
ALTER TABLE tasks_new RENAME TO tasks;
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_created_at ON tasks(created_at DESC);

-- +goose Down
-- SQLite cannot easily revert CHECK changes; same rationale as 0002/0003.
SELECT 1;
