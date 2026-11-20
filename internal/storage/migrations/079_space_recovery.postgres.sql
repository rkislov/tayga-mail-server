-- +goose Up
ALTER TABLE file_spaces ADD COLUMN generation BIGINT NOT NULL DEFAULT 0;
CREATE TABLE file_space_restores (
 id TEXT PRIMARY KEY, space_id TEXT NOT NULL REFERENCES file_spaces(id) ON DELETE CASCADE,
 applied_at BIGINT NOT NULL
);
