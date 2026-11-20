-- +goose Up
ALTER TABLE mailbox_delegates ADD COLUMN can_read_calendar BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE mailbox_delegates ADD COLUMN can_manage_calendar BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE mailbox_delegates ADD COLUMN can_respond_invites BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE mailbox_delegates ADD COLUMN can_manage_mail BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE mailbox_delegates DROP COLUMN can_manage_mail;
ALTER TABLE mailbox_delegates DROP COLUMN can_respond_invites;
ALTER TABLE mailbox_delegates DROP COLUMN can_manage_calendar;
ALTER TABLE mailbox_delegates DROP COLUMN can_read_calendar;
