-- +goose Up
ALTER TABLE xmpp_pep_nodes ADD COLUMN notify_retract BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE xmpp_pep_nodes ADD COLUMN notify_delete BOOLEAN NOT NULL DEFAULT TRUE;
CREATE TABLE xmpp_pep_subscriptions (
    node_id TEXT NOT NULL REFERENCES xmpp_pep_nodes(id) ON DELETE CASCADE,
    jid TEXT NOT NULL,
    subid TEXT NOT NULL,
    PRIMARY KEY (node_id, jid)
);
CREATE INDEX idx_pep_subscriptions_jid ON xmpp_pep_subscriptions(jid);
-- +goose Down
DROP TABLE xmpp_pep_subscriptions;
ALTER TABLE xmpp_pep_nodes DROP COLUMN notify_delete;
ALTER TABLE xmpp_pep_nodes DROP COLUMN notify_retract;
