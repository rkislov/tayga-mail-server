-- +goose Up
ALTER TABLE xmpp_pep_nodes ADD COLUMN max_items INTEGER NOT NULL DEFAULT 0;
ALTER TABLE xmpp_pep_nodes ADD COLUMN send_last BOOLEAN NOT NULL DEFAULT TRUE;
-- +goose Down
ALTER TABLE xmpp_pep_nodes DROP COLUMN send_last;
ALTER TABLE xmpp_pep_nodes DROP COLUMN max_items;
