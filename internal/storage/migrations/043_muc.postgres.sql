-- +goose Up
ALTER TABLE web_chat_members ADD COLUMN affiliation TEXT NOT NULL DEFAULT 'member';
ALTER TABLE web_chat_rooms ADD COLUMN subject TEXT NOT NULL DEFAULT '';
ALTER TABLE web_chat_messages ADD COLUMN nick TEXT NOT NULL DEFAULT '';
ALTER TABLE web_chat_messages ADD COLUMN origin_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX web_chat_message_origin ON web_chat_messages(room_jid,sender,origin_id) WHERE origin_id <> '';
-- +goose Down
DROP INDEX web_chat_message_origin;
ALTER TABLE web_chat_messages DROP COLUMN origin_id;
ALTER TABLE web_chat_messages DROP COLUMN nick;
ALTER TABLE web_chat_rooms DROP COLUMN subject;
ALTER TABLE web_chat_members DROP COLUMN affiliation;
