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
CREATE FUNCTION create_file_user_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO file_owners(id,user_id) VALUES(NEW.id,NEW.id); RETURN NEW; END; $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER file_user_owner AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION create_file_user_owner();
-- +goose StatementEnd
-- +goose StatementBegin
CREATE FUNCTION create_file_space_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO file_owners(id,space_id) VALUES(NEW.id,NEW.id); RETURN NEW; END; $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER file_space_owner AFTER INSERT ON file_spaces FOR EACH ROW EXECUTE FUNCTION create_file_space_owner();
-- +goose StatementEnd
ALTER TABLE file_acl DROP CONSTRAINT file_acl_owner_id_fkey;
ALTER TABLE file_acl ADD CONSTRAINT file_acl_owner_id_fkey FOREIGN KEY(owner_id) REFERENCES file_owners(id) ON DELETE CASCADE;
ALTER TABLE file_shares DROP CONSTRAINT file_shares_user_id_fkey;
ALTER TABLE file_shares ADD CONSTRAINT file_shares_user_id_fkey FOREIGN KEY(user_id) REFERENCES file_owners(id) ON DELETE CASCADE;
