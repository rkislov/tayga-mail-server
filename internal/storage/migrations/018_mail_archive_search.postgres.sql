-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS subject TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS from_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS to_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS date_hdr TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS archived INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_messages_from ON messages(from_addr);
CREATE INDEX IF NOT EXISTS idx_messages_subject ON messages(subject);

CREATE TABLE IF NOT EXISTS message_search (
    message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    subject TEXT NOT NULL DEFAULT '',
    from_addr TEXT NOT NULL DEFAULT '',
    to_addr TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    tsv tsvector
);
CREATE INDEX IF NOT EXISTS idx_message_search_tsv ON message_search USING GIN (tsv);

-- +goose Down
DROP TABLE IF EXISTS message_search;
ALTER TABLE messages DROP COLUMN IF EXISTS archived;
ALTER TABLE messages DROP COLUMN IF EXISTS date_hdr;
ALTER TABLE messages DROP COLUMN IF EXISTS to_addr;
ALTER TABLE messages DROP COLUMN IF EXISTS from_addr;
ALTER TABLE messages DROP COLUMN IF EXISTS subject;
