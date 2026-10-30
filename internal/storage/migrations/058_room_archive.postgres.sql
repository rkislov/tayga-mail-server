-- +goose Up
ALTER TABLE web_chat_rooms ADD COLUMN next_archive_seq BIGINT NOT NULL DEFAULT 0;
ALTER TABLE web_chat_messages ADD COLUMN archive_seq BIGINT NOT NULL DEFAULT 0;
ALTER TABLE web_chat_messages ADD COLUMN sender_jid TEXT NOT NULL DEFAULT '';
-- Fixed precision UTC timestamps sort correctly at whole-second boundaries.
UPDATE web_chat_messages SET created_at=substr(created_at,1,19)||'.'||
 CASE WHEN substr(created_at,20,1)='.' THEN substr(substr(created_at,21,length(created_at)-21)||'000000000',1,9) ELSE '000000000' END||'Z'
 WHERE substr(created_at,length(created_at),1)='Z';
WITH ordered AS (SELECT id,ROW_NUMBER() OVER (PARTITION BY room_jid ORDER BY created_at,id) AS seq FROM web_chat_messages)
UPDATE web_chat_messages SET archive_seq=(SELECT seq FROM ordered WHERE ordered.id=web_chat_messages.id),sender_jid=sender;
UPDATE web_chat_rooms SET next_archive_seq=COALESCE((SELECT MAX(archive_seq) FROM web_chat_messages WHERE room_jid=web_chat_rooms.jid),0);
-- Zero is reserved for messages written by an older binary after rollback.
CREATE UNIQUE INDEX idx_room_archive_seq ON web_chat_messages(room_jid,archive_seq) WHERE archive_seq>0;
CREATE INDEX idx_room_archive_legacy ON web_chat_messages(room_jid) WHERE archive_seq=0;
-- +goose Down
DROP INDEX idx_room_archive_legacy;
DROP INDEX idx_room_archive_seq;
ALTER TABLE web_chat_messages DROP COLUMN sender_jid;
ALTER TABLE web_chat_messages DROP COLUMN archive_seq;
ALTER TABLE web_chat_rooms DROP COLUMN next_archive_seq;
