-- +goose Up
CREATE TABLE IF NOT EXISTS xmpp_bots (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    token_prefix TEXT NOT NULL DEFAULT '',
    webhook_url TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_xmpp_bots_token_prefix ON xmpp_bots(token_prefix);

CREATE TABLE IF NOT EXISTS xmpp_bot_inbox (
    id TEXT PRIMARY KEY,
    bot_id TEXT NOT NULL REFERENCES xmpp_bots(id) ON DELETE CASCADE,
    from_jid TEXT NOT NULL,
    stanza TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_xmpp_bot_inbox_bot ON xmpp_bot_inbox(bot_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS xmpp_bot_inbox;
DROP TABLE IF EXISTS xmpp_bots;
