-- +goose Up
CREATE TABLE tasks (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 source_message_id TEXT UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
 title TEXT NOT NULL,
 body TEXT NOT NULL DEFAULT '',
 completed BOOLEAN NOT NULL DEFAULT FALSE,
 due TEXT NOT NULL DEFAULT '',
 revision BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX tasks_user ON tasks(user_id);
INSERT INTO tasks(id,user_id,source_message_id,title)
 SELECT 'mailtask:' || m.id,b.user_id,m.id,m.subject FROM messages m JOIN mailboxes b ON b.id=m.mailbox_id WHERE m.flags LIKE '%\Flagged%';
-- +goose Down
DROP TABLE tasks;
