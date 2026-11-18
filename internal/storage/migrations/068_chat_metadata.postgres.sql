-- +goose Up
ALTER TABLE chat_posts ADD COLUMN props TEXT NOT NULL DEFAULT '{}';
ALTER TABLE chat_posts ADD COLUMN post_type TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE chat_posts DROP COLUMN post_type;
ALTER TABLE chat_posts DROP COLUMN props;
