-- +goose Up
CREATE TABLE IF NOT EXISTS chat_sessions (
 session_id TEXT PRIMARY KEY REFERENCES oauth_tokens(id) ON DELETE CASCADE,
 tenant_id TEXT NOT NULL,
 domain_id TEXT NOT NULL,
 expires_at BIGINT NOT NULL,
 last_activity_at BIGINT NOT NULL
);
-- +goose Down
DROP TABLE chat_sessions;
