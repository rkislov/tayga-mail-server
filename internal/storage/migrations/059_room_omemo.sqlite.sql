ALTER TABLE web_chat_messages ADD COLUMN omemo TEXT NOT NULL DEFAULT '';
ALTER TABLE web_chat_rooms ADD COLUMN membership_token TEXT NOT NULL DEFAULT '';
