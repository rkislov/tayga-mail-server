-- +goose Up
ALTER TABLE tasks ADD COLUMN completed_at TEXT NOT NULL DEFAULT '';
UPDATE tasks SET completed_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE completed=TRUE;

-- +goose Down
ALTER TABLE tasks DROP COLUMN completed_at;
