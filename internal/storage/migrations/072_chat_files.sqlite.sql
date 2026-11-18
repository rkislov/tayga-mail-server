-- +goose Up
ALTER TABLE chat_posts ADD COLUMN attachments TEXT NOT NULL DEFAULT '[]';
CREATE TABLE chat_files (
 id TEXT PRIMARY KEY,
 channel_id TEXT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 post_id TEXT REFERENCES chat_posts(id) ON DELETE CASCADE,
 created_at BIGINT NOT NULL,
 name TEXT NOT NULL,
 mime_type TEXT NOT NULL,
 size_bytes BIGINT NOT NULL,
 width INTEGER NOT NULL,
 height INTEGER NOT NULL,
 content BLOB NOT NULL
);
CREATE INDEX chat_files_pending ON chat_files(created_at) WHERE post_id IS NULL;
CREATE TABLE chat_file_write_lock (id INTEGER PRIMARY KEY);
INSERT INTO chat_file_write_lock(id) VALUES (1);
-- +goose Down
DROP TABLE chat_file_write_lock;
DROP TABLE chat_files;
ALTER TABLE chat_posts DROP COLUMN attachments;
