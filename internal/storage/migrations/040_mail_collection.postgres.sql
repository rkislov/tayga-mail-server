-- +goose Up
CREATE TABLE mail_sources (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL, protocol TEXT NOT NULL, host TEXT NOT NULL, port BIGINT NOT NULL,
 security TEXT NOT NULL, username TEXT NOT NULL, password_ciphertext BYTEA NOT NULL,
 source_folder TEXT NOT NULL DEFAULT 'INBOX', target_folder TEXT NOT NULL,
 interval_minutes BIGINT NOT NULL, enabled BIGINT NOT NULL DEFAULT 1,
 next_run BIGINT NOT NULL DEFAULT 0, last_run BIGINT NOT NULL DEFAULT 0,
 last_status TEXT NOT NULL DEFAULT 'pending', last_error TEXT NOT NULL DEFAULT '',
 copied BIGINT NOT NULL DEFAULT 0, lease_until BIGINT NOT NULL DEFAULT 0,
 lease_token TEXT NOT NULL DEFAULT '', created_at BIGINT NOT NULL
);
CREATE INDEX mail_sources_due ON mail_sources(enabled,next_run,lease_until);
CREATE INDEX mail_sources_user ON mail_sources(user_id);
CREATE TABLE mail_source_receipts (
 source_id TEXT NOT NULL REFERENCES mail_sources(id) ON DELETE CASCADE,
 remote_key TEXT NOT NULL, created_at BIGINT NOT NULL,
 PRIMARY KEY (source_id,remote_key)
);
-- +goose Down
DROP TABLE mail_source_receipts;
DROP TABLE mail_sources;
