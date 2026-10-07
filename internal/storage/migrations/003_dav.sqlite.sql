-- +goose Up
CREATE TABLE IF NOT EXISTS calendars (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    ctag TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_calendars_user ON calendars(user_id);

CREATE TABLE IF NOT EXISTS calendar_objects (
    id TEXT PRIMARY KEY,
    calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    uid TEXT NOT NULL,
    href_name TEXT NOT NULL,
    etag TEXT NOT NULL,
    data TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    component TEXT NOT NULL DEFAULT 'VEVENT',
    dtstart TIMESTAMP,
    dtend TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(calendar_id, href_name),
    UNIQUE(calendar_id, uid)
);

CREATE INDEX IF NOT EXISTS idx_cal_objects_cal ON calendar_objects(calendar_id);

CREATE TABLE IF NOT EXISTS addressbooks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    ctag TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_addressbooks_user ON addressbooks(user_id);

CREATE TABLE IF NOT EXISTS address_objects (
    id TEXT PRIMARY KEY,
    addressbook_id TEXT NOT NULL REFERENCES addressbooks(id) ON DELETE CASCADE,
    uid TEXT NOT NULL,
    href_name TEXT NOT NULL,
    etag TEXT NOT NULL,
    data TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(addressbook_id, href_name),
    UNIQUE(addressbook_id, uid)
);

CREATE INDEX IF NOT EXISTS idx_addr_objects_ab ON address_objects(addressbook_id);

-- +goose Down
DROP TABLE IF EXISTS address_objects;
DROP TABLE IF EXISTS addressbooks;
DROP TABLE IF EXISTS calendar_objects;
DROP TABLE IF EXISTS calendars;
