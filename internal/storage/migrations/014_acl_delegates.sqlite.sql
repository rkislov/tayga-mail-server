-- +goose Up
CREATE TABLE IF NOT EXISTS mailbox_delegates (
    owner_id TEXT NOT NULL,
    delegate_id TEXT NOT NULL,
    can_read INTEGER NOT NULL DEFAULT 1,
    can_send_as INTEGER NOT NULL DEFAULT 0,
    can_send_on_behalf INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (owner_id, delegate_id),
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (delegate_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS mailbox_acl (
    mailbox_id TEXT NOT NULL,
    grantee_user_id TEXT NOT NULL,
    rights TEXT NOT NULL DEFAULT 'lr',
    PRIMARY KEY (mailbox_id, grantee_user_id),
    FOREIGN KEY (mailbox_id) REFERENCES mailboxes(id) ON DELETE CASCADE,
    FOREIGN KEY (grantee_user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS calendar_acl (
    calendar_id TEXT NOT NULL,
    grantee_user_id TEXT NOT NULL,
    rights TEXT NOT NULL DEFAULT 'read',
    PRIMARY KEY (calendar_id, grantee_user_id),
    FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON DELETE CASCADE,
    FOREIGN KEY (grantee_user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS calendar_acl;
DROP TABLE IF EXISTS mailbox_acl;
DROP TABLE IF EXISTS mailbox_delegates;
