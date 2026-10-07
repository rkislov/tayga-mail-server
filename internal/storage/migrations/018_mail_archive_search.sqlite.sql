-- +goose Up
ALTER TABLE messages ADD COLUMN subject TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN from_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN to_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN date_hdr TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_messages_from ON messages(from_addr);
CREATE INDEX IF NOT EXISTS idx_messages_subject ON messages(subject);

CREATE VIRTUAL TABLE IF NOT EXISTS message_fts USING fts5(
    message_id UNINDEXED,
    subject,
    from_addr,
    to_addr,
    body,
    tokenize = 'unicode61'
);

-- +goose Down
DROP TABLE IF EXISTS message_fts;
