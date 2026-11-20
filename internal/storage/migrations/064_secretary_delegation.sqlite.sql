-- +goose Up
ALTER TABLE mailbox_delegates ADD COLUMN can_read_calendar INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mailbox_delegates ADD COLUMN can_manage_calendar INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mailbox_delegates ADD COLUMN can_respond_invites INTEGER NOT NULL DEFAULT 0;

ALTER TABLE mailbox_delegates ADD COLUMN can_manage_mail INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE mailbox_delegates DROP COLUMN can_manage_mail;
ALTER TABLE mailbox_delegates DROP COLUMN can_respond_invites;
ALTER TABLE mailbox_delegates DROP COLUMN can_manage_calendar;
ALTER TABLE mailbox_delegates DROP COLUMN can_read_calendar;
