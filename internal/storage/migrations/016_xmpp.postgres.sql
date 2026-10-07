-- +goose Up
CREATE TABLE IF NOT EXISTS xmpp_roster (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    jid TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    subscription TEXT NOT NULL DEFAULT 'none',
    groups TEXT NOT NULL DEFAULT '[]',
    ask TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, jid)
);
CREATE INDEX IF NOT EXISTS idx_xmpp_roster_user ON xmpp_roster(user_id);

CREATE TABLE IF NOT EXISTS xmpp_offline (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stanza TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_xmpp_offline_user ON xmpp_offline(user_id, created_at);

CREATE TABLE IF NOT EXISTS xmpp_mam (
    id TEXT PRIMARY KEY,
    owner_bare_jid TEXT NOT NULL,
    with_bare_jid TEXT NOT NULL DEFAULT '',
    stanza_id TEXT NOT NULL,
    stanza TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_xmpp_mam_owner_time ON xmpp_mam(owner_bare_jid, created_at);
CREATE INDEX IF NOT EXISTS idx_xmpp_mam_with ON xmpp_mam(owner_bare_jid, with_bare_jid, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_xmpp_mam_stanza ON xmpp_mam(owner_bare_jid, stanza_id);

CREATE TABLE IF NOT EXISTS xmpp_pep_nodes (
    id TEXT PRIMARY KEY,
    owner_bare_jid TEXT NOT NULL,
    node TEXT NOT NULL,
    access_model TEXT NOT NULL DEFAULT 'presence',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(owner_bare_jid, node)
);

CREATE TABLE IF NOT EXISTS xmpp_pep_items (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES xmpp_pep_nodes(id) ON DELETE CASCADE,
    item_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(node_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_xmpp_pep_items_node ON xmpp_pep_items(node_id);

-- +goose Down
DROP TABLE IF EXISTS xmpp_pep_items;
DROP TABLE IF EXISTS xmpp_pep_nodes;
DROP TABLE IF EXISTS xmpp_mam;
DROP TABLE IF EXISTS xmpp_offline;
DROP TABLE IF EXISTS xmpp_roster;
