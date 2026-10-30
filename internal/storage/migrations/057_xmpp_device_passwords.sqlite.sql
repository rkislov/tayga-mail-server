-- +goose Up
CREATE TABLE xmpp_device_passwords (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 secret_hash TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL,
 last_used_at TIMESTAMP,
 revoked_at TIMESTAMP
);
CREATE INDEX idx_xmpp_device_passwords_user ON xmpp_device_passwords(user_id);
CREATE TABLE xmpp_auth_policy (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 devices_only BOOLEAN NOT NULL DEFAULT FALSE
);
ALTER TABLE xmpp_push_registrations ADD COLUMN credential_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_xmpp_push_credential ON xmpp_push_registrations(user_id,credential_id);
-- +goose Down
DROP INDEX idx_xmpp_push_credential;
ALTER TABLE xmpp_push_registrations DROP COLUMN credential_id;
DROP TABLE xmpp_auth_policy;
DROP TABLE xmpp_device_passwords;
