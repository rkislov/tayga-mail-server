-- +goose Up
CREATE TABLE calendar_occurrence_proposals (
    invite_id TEXT NOT NULL REFERENCES calendar_invites(id) ON DELETE CASCADE,
    recurrence_id TEXT NOT NULL,
    object_id TEXT NOT NULL REFERENCES calendar_objects(id) ON DELETE CASCADE,
    revision TEXT NOT NULL,
    event_sequence INTEGER NOT NULL,
    original_start TIMESTAMP NOT NULL,
    original_end TIMESTAMP NOT NULL,
    proposed_start TIMESTAMP NOT NULL,
    proposed_end TIMESTAMP NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (invite_id, recurrence_id)
);
-- +goose Down
DROP TABLE calendar_occurrence_proposals;
