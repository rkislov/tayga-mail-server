-- +goose Up
CREATE TABLE IF NOT EXISTS message_header_bytes (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 subject BYTEA,
 from_addr BYTEA,
 to_addr BYTEA
);
-- +goose Down
-- Preserve original header bytes across schema downgrades.
