-- +goose Up
ALTER TABLE calendar_deliveries ADD COLUMN target_mailbox TEXT NOT NULL DEFAULT 'INBOX' CHECK (target_mailbox IN ('INBOX', 'Sent'));
-- +goose Down
ALTER TABLE calendar_deliveries DROP COLUMN target_mailbox;
