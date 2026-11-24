-- +goose Up
CREATE TABLE chat_federation_peers (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE, owner_id TEXT NOT NULL REFERENCES users(id),
 name TEXT NOT NULL, mode TEXT NOT NULL CHECK(mode IN ('native','rest')), site_url TEXT NOT NULL,
 secret TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 0, confirmed INTEGER NOT NULL DEFAULT 0,
 created_at BIGINT NOT NULL, last_success_at BIGINT NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX chat_federation_peer_name ON chat_federation_peers(domain_id,name) WHERE enabled=1;
CREATE TABLE chat_federation_links (
 id TEXT PRIMARY KEY, peer_id TEXT NOT NULL REFERENCES chat_federation_peers(id) ON DELETE CASCADE,
 channel_id TEXT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE, remote_channel_id TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 0, readonly INTEGER NOT NULL DEFAULT 0, local_readonly INTEGER NOT NULL DEFAULT 0, created_at BIGINT NOT NULL,
 local_cursor_at BIGINT NOT NULL DEFAULT 0, local_cursor_id TEXT NOT NULL DEFAULT '',
 remote_cursor_at BIGINT NOT NULL DEFAULT 0, remote_cursor_id TEXT NOT NULL DEFAULT '',
 lease_token TEXT NOT NULL DEFAULT '', lease_until BIGINT NOT NULL DEFAULT 0,
 UNIQUE(peer_id,channel_id), UNIQUE(peer_id,remote_channel_id)
);
CREATE INDEX chat_federation_readonly ON chat_federation_links(channel_id,enabled,local_readonly);
CREATE TABLE chat_federation_users (
 peer_id TEXT NOT NULL REFERENCES chat_federation_peers(id) ON DELETE CASCADE,
 remote_user_id TEXT NOT NULL, local_user_id TEXT NOT NULL REFERENCES users(id),
 PRIMARY KEY(peer_id,remote_user_id), UNIQUE(local_user_id)
);
CREATE TABLE chat_federation_posts (
 link_id TEXT NOT NULL REFERENCES chat_federation_links(id) ON DELETE CASCADE,
 remote_post_id TEXT NOT NULL, local_post_id TEXT NOT NULL, origin TEXT NOT NULL CHECK(origin IN ('local','remote')),
 last_update_at BIGINT NOT NULL DEFAULT 0, deleted_at BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(link_id,remote_post_id), UNIQUE(link_id,local_post_id)
);
CREATE TABLE chat_federation_invites (
 peer_id TEXT NOT NULL REFERENCES chat_federation_peers(id) ON DELETE CASCADE,
 remote_channel_id TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL, created_at BIGINT NOT NULL,
 PRIMARY KEY(peer_id,remote_channel_id)
);
CREATE TABLE chat_federation_files (
 upload_id TEXT PRIMARY KEY, peer_id TEXT NOT NULL REFERENCES chat_federation_peers(id) ON DELETE CASCADE,
 link_id TEXT NOT NULL REFERENCES chat_federation_links(id) ON DELETE CASCADE,
 remote_file_id TEXT NOT NULL, user_id TEXT NOT NULL REFERENCES users(id), filename TEXT NOT NULL,
 size BIGINT NOT NULL, created_at BIGINT NOT NULL, local_file_id TEXT NOT NULL DEFAULT '',
 UNIQUE(link_id,remote_file_id)
);
CREATE TABLE chat_federation_attempts (
 link_id TEXT NOT NULL REFERENCES chat_federation_links(id) ON DELETE CASCADE,
 local_post_id TEXT NOT NULL, created_at BIGINT NOT NULL, files TEXT NOT NULL DEFAULT '[]',
 PRIMARY KEY(link_id,local_post_id)
);
-- +goose Down
DROP TABLE chat_federation_attempts;
DROP TABLE chat_federation_files;
DROP TABLE chat_federation_invites;
DROP TABLE chat_federation_posts;
DROP TABLE chat_federation_users;
DROP TABLE chat_federation_links;
DROP TABLE chat_federation_peers;
