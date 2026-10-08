-- +goose Up
CREATE TABLE IF NOT EXISTS note_folder_acl (
    folder_id TEXT NOT NULL,
    grantee_user_id TEXT NOT NULL,
    rights TEXT NOT NULL DEFAULT 'write',
    PRIMARY KEY (folder_id, grantee_user_id),
    FOREIGN KEY (folder_id) REFERENCES note_folders(id) ON DELETE CASCADE,
    FOREIGN KEY (grantee_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_acl (
    note_id TEXT NOT NULL,
    grantee_user_id TEXT NOT NULL,
    rights TEXT NOT NULL DEFAULT 'write',
    PRIMARY KEY (note_id, grantee_user_id),
    FOREIGN KEY (note_id) REFERENCES note_items(id) ON DELETE CASCADE,
    FOREIGN KEY (grantee_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_folder_acl_grantee ON note_folder_acl(grantee_user_id);
CREATE INDEX IF NOT EXISTS idx_note_acl_grantee ON note_acl(grantee_user_id);

-- +goose Down
DROP TABLE IF EXISTS note_acl;
DROP TABLE IF EXISTS note_folder_acl;
