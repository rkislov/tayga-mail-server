-- +goose Up
CREATE TABLE IF NOT EXISTS calendar_attachments (
    id TEXT PRIMARY KEY,
    calendar_object_id TEXT NOT NULL REFERENCES calendar_objects(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size INTEGER NOT NULL DEFAULT 0,
    storage_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cal_attach_object ON calendar_attachments(calendar_object_id);
CREATE INDEX IF NOT EXISTS idx_cal_attach_user ON calendar_attachments(user_id);

-- +goose Down
DROP TABLE IF EXISTS calendar_attachments;
