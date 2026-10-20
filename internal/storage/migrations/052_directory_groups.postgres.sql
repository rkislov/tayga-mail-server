CREATE TABLE directory_groups (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    parent_id TEXT REFERENCES directory_groups(id) ON DELETE RESTRICT,
    local_part TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    UNIQUE(domain_id, local_part)
);
CREATE INDEX idx_directory_groups_parent ON directory_groups(parent_id);
CREATE TABLE directory_group_members (
    group_id TEXT NOT NULL REFERENCES directory_groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY(group_id, user_id)
);
CREATE INDEX idx_directory_group_members_user ON directory_group_members(user_id);
