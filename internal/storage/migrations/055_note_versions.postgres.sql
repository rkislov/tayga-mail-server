-- +goose Up
CREATE TABLE note_versions (
    id TEXT PRIMARY KEY,
    note_id TEXT NOT NULL REFERENCES note_items(id) ON DELETE CASCADE,
    etag TEXT NOT NULL,
    snapshot_json TEXT NOT NULL,
    saved_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(note_id, etag)
);
CREATE INDEX idx_note_versions_note ON note_versions(note_id, saved_at);
-- +goose Down
DROP TABLE note_versions;
