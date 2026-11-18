-- +goose Up
CREATE TABLE chat_channels (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 name TEXT NOT NULL, display_name TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('O','P','D')),
 creator_id TEXT NOT NULL REFERENCES users(id), created_at BIGINT NOT NULL,
 UNIQUE(tenant_id, name)
);
CREATE TABLE chat_members (
 channel_id TEXT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 last_viewed_at BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(channel_id, user_id)
);
CREATE TABLE chat_posts (
 id TEXT PRIMARY KEY, channel_id TEXT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id), root_id TEXT NOT NULL DEFAULT '',
 message TEXT NOT NULL, created_at BIGINT NOT NULL,
 pending_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX chat_posts_history ON chat_posts(channel_id, created_at, id);
CREATE INDEX chat_posts_thread ON chat_posts(root_id, created_at, id);
CREATE INDEX chat_members_user ON chat_members(user_id, channel_id);
CREATE UNIQUE INDEX chat_posts_pending ON chat_posts(user_id, pending_id) WHERE pending_id <> '';
-- +goose Down
DROP TABLE chat_posts;
DROP TABLE chat_members;
DROP TABLE chat_channels;
