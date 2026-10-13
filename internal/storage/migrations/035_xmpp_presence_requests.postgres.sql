-- +goose Up
CREATE TABLE xmpp_presence_requests(owner_bare_jid TEXT NOT NULL,from_bare_jid TEXT NOT NULL,PRIMARY KEY(owner_bare_jid,from_bare_jid));
-- +goose Down
DROP TABLE xmpp_presence_requests;
