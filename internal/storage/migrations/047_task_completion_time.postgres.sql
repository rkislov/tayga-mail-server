-- +goose Up
ALTER TABLE tasks ADD COLUMN completed_at TEXT NOT NULL DEFAULT '';
UPDATE tasks SET completed_at=to_char(CURRENT_TIMESTAMP AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"') WHERE completed=TRUE;

-- +goose Down
ALTER TABLE tasks DROP COLUMN completed_at;
