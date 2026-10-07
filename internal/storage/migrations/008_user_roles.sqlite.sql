-- +goose Up
ALTER TABLE users ADD COLUMN roles TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot DROP COLUMN portably; leave column.
