-- +goose Up
CREATE TABLE IF NOT EXISTS user_mfa (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    totp_secret TEXT NOT NULL DEFAULT '',
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    backup_codes TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL UNIQUE,
    refresh_token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_user ON oauth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_access ON oauth_tokens(access_token);

CREATE TABLE IF NOT EXISTS mfa_challenges (
    token TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS mfa_challenges;
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS user_mfa;
