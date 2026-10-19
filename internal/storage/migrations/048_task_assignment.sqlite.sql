-- +goose Up
ALTER TABLE tasks ADD COLUMN assignee_id TEXT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN assignment_state TEXT NOT NULL DEFAULT '' CHECK (assignment_state IN ('','pending','accepted','declined'));
ALTER TABLE tasks ADD COLUMN assignment_revision BIGINT NOT NULL DEFAULT 0;
CREATE INDEX tasks_assignee ON tasks(assignee_id, assignment_state);
CREATE TABLE task_events (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
 revision BIGINT NOT NULL,
 actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
 actor_name TEXT NOT NULL,
 action TEXT NOT NULL,
 detail TEXT NOT NULL,
 created_at TEXT NOT NULL,
 UNIQUE(task_id, revision)
);
CREATE TABLE task_notifications (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL,
 owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 action TEXT NOT NULL,
 created_at TEXT NOT NULL,
 read_at TEXT NOT NULL DEFAULT '',
 dispatched BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX task_notifications_user ON task_notifications(user_id, created_at);
CREATE INDEX task_notifications_pending ON task_notifications(dispatched, created_at);
-- +goose Down
DROP TABLE task_notifications;
DROP TABLE task_events;
DROP INDEX tasks_assignee;
ALTER TABLE tasks DROP COLUMN assignment_revision;
ALTER TABLE tasks DROP COLUMN assignment_state;
ALTER TABLE tasks DROP COLUMN assignee_id;
