-- +goose Up
CREATE TABLE chat_push_receipts (
 id TEXT PRIMARY KEY,
 session_id TEXT NOT NULL REFERENCES chat_push_devices(session_id) ON DELETE CASCADE,
 post_id TEXT REFERENCES chat_posts(id) ON DELETE CASCADE,
 created_at BIGINT NOT NULL, received_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX chat_push_receipts_created ON chat_push_receipts(created_at);
-- +goose Down
DROP TABLE chat_push_receipts;
