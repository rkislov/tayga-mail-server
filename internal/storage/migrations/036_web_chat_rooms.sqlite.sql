-- +goose Up
CREATE TABLE web_chat_rooms (jid TEXT PRIMARY KEY, name TEXT NOT NULL, owner_email TEXT NOT NULL);
CREATE TABLE web_chat_members (room_jid TEXT NOT NULL REFERENCES web_chat_rooms(jid) ON DELETE CASCADE, email TEXT NOT NULL, PRIMARY KEY(room_jid,email));
CREATE INDEX web_chat_members_email ON web_chat_members(email);
CREATE TABLE web_chat_messages (id TEXT PRIMARY KEY, room_jid TEXT NOT NULL REFERENCES web_chat_rooms(jid) ON DELETE CASCADE, sender TEXT NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX web_chat_messages_room ON web_chat_messages(room_jid,created_at);
-- +goose Down
DROP TABLE web_chat_messages;
DROP TABLE web_chat_members;
DROP TABLE web_chat_rooms;
