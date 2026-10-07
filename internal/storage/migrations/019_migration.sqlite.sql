-- +goose Up
ALTER TABLE domains ADD COLUMN migration_enabled TEXT NOT NULL DEFAULT 'inherit';
ALTER TABLE users ADD COLUMN migration_enabled TEXT NOT NULL DEFAULT 'inherit';

CREATE TABLE IF NOT EXISTS migration_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    host TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    tls INTEGER NOT NULL DEFAULT 1,
    username TEXT NOT NULL DEFAULT '',
    password_ciphertext BLOB,
    options TEXT NOT NULL DEFAULT '{}',
    copied INTEGER NOT NULL DEFAULT 0,
    skipped INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    cursor_json TEXT NOT NULL DEFAULT '{}',
    schedule TEXT NOT NULL DEFAULT '',
    next_run_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_migration_jobs_user ON migration_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_migration_jobs_status ON migration_jobs(status);

-- +goose Down
DROP TABLE IF EXISTS migration_jobs;
