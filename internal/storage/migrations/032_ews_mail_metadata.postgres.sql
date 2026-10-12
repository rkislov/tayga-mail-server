-- +goose Up
CREATE TABLE ews_mail_metadata (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 data TEXT NOT NULL
);
-- +goose Down
DROP TABLE ews_mail_metadata;
