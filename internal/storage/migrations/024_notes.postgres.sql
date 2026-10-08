-- +goose Up
CREATE TABLE IF NOT EXISTS note_folders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    parent_id TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    ctag TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_note_folders_user ON note_folders(user_id);

CREATE TABLE IF NOT EXISTS note_items (
    id TEXT PRIMARY KEY,
    folder_id TEXT NOT NULL REFERENCES note_folders(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    document_json TEXT NOT NULL DEFAULT '',
    body_html TEXT NOT NULL DEFAULT '',
    body_text TEXT NOT NULL DEFAULT '',
    pinned INTEGER NOT NULL DEFAULT 0,
    archived INTEGER NOT NULL DEFAULT 0,
    color TEXT NOT NULL DEFAULT '',
    etag TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_note_items_folder ON note_items(folder_id);
CREATE INDEX IF NOT EXISTS idx_note_items_user ON note_items(user_id);
CREATE INDEX IF NOT EXISTS idx_note_items_updated ON note_items(folder_id, updated_at);

CREATE TABLE IF NOT EXISTS note_attachments (
    id TEXT PRIMARY KEY,
    note_id TEXT NOT NULL REFERENCES note_items(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size INTEGER NOT NULL DEFAULT 0,
    storage_path TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL DEFAULT 'file',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_note_attach_note ON note_attachments(note_id);

-- +goose Down
DROP TABLE IF EXISTS note_attachments;
DROP TABLE IF EXISTS note_items;
DROP TABLE IF EXISTS note_folders;
