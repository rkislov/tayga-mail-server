-- +goose Up
ALTER TABLE mailbox_delegates ADD COLUMN ews_inbox_permission TEXT NOT NULL DEFAULT '';
ALTER TABLE mailbox_delegates ADD COLUMN ews_calendar_permission TEXT NOT NULL DEFAULT '';
ALTER TABLE mailbox_delegates ADD COLUMN ews_receive_meetings BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE mailbox_delegates DROP COLUMN ews_receive_meetings;
ALTER TABLE mailbox_delegates DROP COLUMN ews_calendar_permission;
ALTER TABLE mailbox_delegates DROP COLUMN ews_inbox_permission;
