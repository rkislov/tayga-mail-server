-- +goose Up
CREATE TABLE IF NOT EXISTS greylist (
    id TEXT PRIMARY KEY,
    client_ip TEXT NOT NULL,
    envelope_from TEXT NOT NULL DEFAULT '',
    rcpt TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    passed BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(client_ip, envelope_from, rcpt)
);

CREATE INDEX IF NOT EXISTS idx_greylist_last_seen ON greylist(last_seen);

-- +goose Down
DROP TABLE IF EXISTS greylist;
