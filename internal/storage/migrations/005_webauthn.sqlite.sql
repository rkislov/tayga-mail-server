-- +goose Up
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    credential_json TEXT NOT NULL,
    sign_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user ON webauthn_credentials(user_id);

CREATE TABLE IF NOT EXISTS webauthn_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    session_json TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webauthn_sessions_user ON webauthn_sessions(user_id);

-- +goose Down
DROP TABLE IF EXISTS webauthn_sessions;
DROP TABLE IF EXISTS webauthn_credentials;
