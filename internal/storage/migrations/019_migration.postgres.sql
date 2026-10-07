-- +goose Up
ALTER TABLE domains ADD COLUMN IF NOT EXISTS migration_enabled TEXT NOT NULL DEFAULT 'inherit';
ALTER TABLE users ADD COLUMN IF NOT EXISTS migration_enabled TEXT NOT NULL DEFAULT 'inherit';

CREATE TABLE IF NOT EXISTS migration_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    host TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    tls BOOLEAN NOT NULL DEFAULT TRUE,
    username TEXT NOT NULL DEFAULT '',
    password_ciphertext BYTEA,
    options TEXT NOT NULL DEFAULT '{}',
    copied BIGINT NOT NULL DEFAULT 0,
    skipped BIGINT NOT NULL DEFAULT 0,
    errors BIGINT NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    cursor_json TEXT NOT NULL DEFAULT '{}',
    schedule TEXT NOT NULL DEFAULT '',
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_migration_jobs_user ON migration_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_migration_jobs_status ON migration_jobs(status);

-- +goose Down
DROP TABLE IF EXISTS migration_jobs;
ALTER TABLE users DROP COLUMN IF EXISTS migration_enabled;
ALTER TABLE domains DROP COLUMN IF EXISTS migration_enabled;
