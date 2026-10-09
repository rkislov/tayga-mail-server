-- +goose Up
CREATE TABLE file_acl(owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,path TEXT NOT NULL,grantee_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,rights TEXT NOT NULL,PRIMARY KEY(owner_id,path,grantee_id));
CREATE INDEX file_acl_grantee ON file_acl(grantee_id);
-- +goose Down
DROP TABLE file_acl;
