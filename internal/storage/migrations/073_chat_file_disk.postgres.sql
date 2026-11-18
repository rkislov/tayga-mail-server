-- +goose Up
ALTER TABLE chat_files ADD COLUMN on_disk INTEGER NOT NULL DEFAULT 0;
-- +goose Down
ALTER TABLE chat_files DROP COLUMN on_disk;
