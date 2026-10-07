-- +goose Up
CREATE TABLE IF NOT EXISTS outbound_queue (
    id TEXT PRIMARY KEY,
    envelope_from TEXT NOT NULL,
    envelope_to TEXT NOT NULL,
    message_id TEXT NOT NULL DEFAULT '',
    data BLOB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 8,
    next_attempt_at TIMESTAMP NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_outbound_next ON outbound_queue(next_attempt_at);

-- +goose Down
DROP TABLE IF EXISTS outbound_queue;
