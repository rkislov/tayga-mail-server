-- +goose Up
CREATE TABLE calendar_schedule_tags (
 object_id TEXT PRIMARY KEY REFERENCES calendar_objects(id) ON DELETE CASCADE,
 tag TEXT NOT NULL
);
INSERT INTO calendar_schedule_tags(object_id,tag) SELECT id,etag FROM calendar_objects;
CREATE TABLE calendar_scheduling_inbox (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 data TEXT NOT NULL,
 etag TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX calendar_scheduling_inbox_user ON calendar_scheduling_inbox(user_id,created_at,id);
-- +goose Down
DROP TABLE calendar_scheduling_inbox;
DROP TABLE calendar_schedule_tags;
