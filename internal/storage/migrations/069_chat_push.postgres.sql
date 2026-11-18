-- +goose Up
CREATE TABLE chat_push_devices (
 session_id TEXT PRIMARY KEY REFERENCES oauth_tokens(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 tenant_id TEXT NOT NULL, domain_id TEXT NOT NULL,
 device_id TEXT NOT NULL UNIQUE, disabled INTEGER NOT NULL DEFAULT 0,
 expires_at BIGINT NOT NULL, updated_at BIGINT NOT NULL
);
CREATE TABLE chat_push_jobs (
 post_id TEXT NOT NULL REFERENCES chat_posts(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL REFERENCES chat_push_devices(session_id) ON DELETE CASCADE,
 attempts INTEGER NOT NULL DEFAULT 0, next_at BIGINT NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending', result TEXT NOT NULL DEFAULT '', finished_at BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(post_id,session_id)
);
CREATE INDEX chat_push_due ON chat_push_jobs(state,next_at);
-- +goose Down
DROP TABLE chat_push_jobs;
DROP TABLE chat_push_devices;
