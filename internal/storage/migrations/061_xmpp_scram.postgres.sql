CREATE TABLE xmpp_scram_accounts (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 salt TEXT NOT NULL
);
CREATE TABLE xmpp_scram_keys (
 user_id TEXT NOT NULL REFERENCES xmpp_scram_accounts(user_id) ON DELETE CASCADE,
 credential_id TEXT NOT NULL,
 source_hash TEXT NOT NULL,
 stored_key TEXT NOT NULL,
 server_key TEXT NOT NULL,
 PRIMARY KEY(user_id, credential_id)
);
