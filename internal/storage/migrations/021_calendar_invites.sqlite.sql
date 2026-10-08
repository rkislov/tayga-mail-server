-- +goose Up
CREATE TABLE IF NOT EXISTS calendar_invites (
    id TEXT PRIMARY KEY,
    organizer_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    attendee_email TEXT NOT NULL DEFAULT '',
    attendee_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    event_uid TEXT NOT NULL DEFAULT '',
    calendar_object_id TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    partstat TEXT NOT NULL DEFAULT 'NEEDS-ACTION',
    proposed_start TIMESTAMP,
    proposed_end TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cal_invites_attendee_user ON calendar_invites(attendee_user_id);
CREATE INDEX IF NOT EXISTS idx_cal_invites_event_uid ON calendar_invites(event_uid);
CREATE INDEX IF NOT EXISTS idx_cal_invites_organizer ON calendar_invites(organizer_user_id);

-- +goose Down
DROP TABLE IF EXISTS calendar_invites;
