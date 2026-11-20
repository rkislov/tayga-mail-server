-- +goose Up
CREATE TABLE file_spaces (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 name TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('department','project')),
 description TEXT NOT NULL DEFAULT '', quota_bytes BIGINT NOT NULL DEFAULT 0 CHECK(quota_bytes>=0),
 version_days INTEGER NOT NULL DEFAULT 0, trash_days INTEGER NOT NULL DEFAULT 0,
 keep_versions INTEGER NOT NULL DEFAULT 0, automatic BOOLEAN NOT NULL DEFAULT FALSE,
 revision BIGINT NOT NULL DEFAULT 1, created_at BIGINT NOT NULL
);
CREATE INDEX file_spaces_tenant ON file_spaces(tenant_id);
CREATE TABLE file_space_groups (
 space_id TEXT NOT NULL REFERENCES file_spaces(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES directory_groups(id) ON DELETE CASCADE,
 rights TEXT NOT NULL CHECK(rights IN ('read','write','manage')),
 PRIMARY KEY(space_id,group_id)
);
CREATE TABLE file_owners (
 id TEXT PRIMARY KEY,
 user_id TEXT UNIQUE REFERENCES users(id) ON DELETE CASCADE,
 space_id TEXT UNIQUE REFERENCES file_spaces(id) ON DELETE CASCADE,
 CHECK((user_id IS NOT NULL AND space_id IS NULL) OR (user_id IS NULL AND space_id IS NOT NULL)),
 CHECK(id=COALESCE(user_id,space_id))
);
INSERT INTO file_owners(id,user_id) SELECT id,id FROM users;
-- +goose StatementBegin
CREATE TRIGGER file_user_owner AFTER INSERT ON users BEGIN INSERT INTO file_owners(id,user_id) VALUES(NEW.id,NEW.id); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER file_space_owner AFTER INSERT ON file_spaces BEGIN INSERT INTO file_owners(id,space_id) VALUES(NEW.id,NEW.id); END;
-- +goose StatementEnd
CREATE TABLE file_acl_new(owner_id TEXT NOT NULL REFERENCES file_owners(id) ON DELETE CASCADE,path TEXT NOT NULL,grantee_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,rights TEXT NOT NULL,PRIMARY KEY(owner_id,path,grantee_id));
INSERT INTO file_acl_new SELECT * FROM file_acl;
DROP TABLE file_acl;
ALTER TABLE file_acl_new RENAME TO file_acl;
CREATE INDEX file_acl_grantee ON file_acl(grantee_id);
CREATE TABLE file_shares_new (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES file_owners(id) ON DELETE CASCADE,
 path TEXT NOT NULL, token TEXT NOT NULL UNIQUE, expires_at TIMESTAMP,
 max_downloads INTEGER NOT NULL DEFAULT 0, download_count INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, password_hash TEXT NOT NULL DEFAULT '',rights TEXT NOT NULL DEFAULT 'read'
);
INSERT INTO file_shares_new SELECT id,user_id,path,token,expires_at,max_downloads,download_count,created_at,password_hash,rights FROM file_shares;
DROP TABLE file_shares;
ALTER TABLE file_shares_new RENAME TO file_shares;
CREATE INDEX idx_file_shares_token ON file_shares(token);
CREATE INDEX idx_file_shares_user ON file_shares(user_id);
