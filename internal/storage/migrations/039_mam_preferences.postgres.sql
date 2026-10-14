-- +goose Up
CREATE TABLE xmpp_mam_preferences (owner_bare_jid TEXT PRIMARY KEY, preferences TEXT NOT NULL);
-- +goose Down
DROP TABLE xmpp_mam_preferences;
