-- +goose Up
CREATE TABLE IF NOT EXISTS message_header_bytes (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 subject BLOB,
 from_addr BLOB,
 to_addr BLOB
);
-- +goose Down
-- Preserve original header bytes across schema downgrades.
