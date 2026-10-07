-- +goose Up
CREATE TABLE IF NOT EXISTS user_writer_leases (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_writer_leases_expires ON user_writer_leases(expires_at);

-- +goose Down
DROP TABLE IF EXISTS user_writer_leases;
