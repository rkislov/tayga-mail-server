-- +goose Up
-- Retain the most recently updated invitation for each organizer/meeting/attendee.
DELETE FROM calendar_invites WHERE id IN (
 SELECT old.id FROM calendar_invites old JOIN calendar_invites newer
 ON old.organizer_user_id=newer.organizer_user_id AND old.event_uid=newer.event_uid
 AND old.attendee_email=newer.attendee_email
 WHERE old.updated_at < newer.updated_at OR (old.updated_at=newer.updated_at AND old.id < newer.id)
);
CREATE UNIQUE INDEX calendar_invite_identity ON calendar_invites(organizer_user_id, event_uid, attendee_email);
-- +goose Down
DROP INDEX calendar_invite_identity;
