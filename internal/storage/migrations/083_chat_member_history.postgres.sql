-- +goose Up
CREATE TABLE chat_member_history (
 id TEXT PRIMARY KEY,
 channel_id TEXT NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 joined_at BIGINT NOT NULL,
 left_at BIGINT
);
CREATE UNIQUE INDEX chat_member_history_active ON chat_member_history(channel_id,user_id) WHERE left_at IS NULL;
CREATE INDEX chat_member_history_channel ON chat_member_history(channel_id,joined_at);
-- Existing memberships have no recorded join date. Zero explicitly means
-- present before history collection started. Do not invent an old timestamp.
INSERT INTO chat_member_history(id,channel_id,user_id,joined_at)
 SELECT channel_id||':'||user_id,channel_id,user_id,0 FROM chat_members;
-- +goose Down
DROP TABLE chat_member_history;
