-- +goose Up
CREATE TABLE IF NOT EXISTS mailbox_delegates (
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delegate_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_read BOOLEAN NOT NULL DEFAULT TRUE,
    can_send_as BOOLEAN NOT NULL DEFAULT FALSE,
    can_send_on_behalf BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (owner_id, delegate_id)
);
CREATE TABLE IF NOT EXISTS mailbox_acl (
    mailbox_id TEXT NOT NULL REFERENCES mailboxes(id) ON DELETE CASCADE,
    grantee_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rights TEXT NOT NULL DEFAULT 'lr',
    PRIMARY KEY (mailbox_id, grantee_user_id)
);
CREATE TABLE IF NOT EXISTS calendar_acl (
    calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    grantee_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rights TEXT NOT NULL DEFAULT 'read',
    PRIMARY KEY (calendar_id, grantee_user_id)
);

-- +goose Down
DROP TABLE IF EXISTS calendar_acl;
DROP TABLE IF EXISTS mailbox_acl;
DROP TABLE IF EXISTS mailbox_delegates;
