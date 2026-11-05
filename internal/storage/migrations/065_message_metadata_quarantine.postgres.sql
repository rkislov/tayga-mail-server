-- +goose Up
CREATE TABLE IF NOT EXISTS storage_orphaned_message_metadata (
 source_table TEXT NOT NULL,
 message_id TEXT NOT NULL,
 row_data TEXT NOT NULL,
 quarantined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(source_table,message_id),
 CHECK(source_table IN ('travel_jobs','ews_mail_metadata'))
);
-- Preserve every removed value without linking the archive to missing parents.
INSERT INTO storage_orphaned_message_metadata(source_table,message_id,row_data)
SELECT 'travel_jobs',j.message_id,json_build_object('message_id',j.message_id,'user_id',j.user_id,'state',j.state,'lease_token',j.lease_token,'lease_until',j.lease_until,'next_attempt',j.next_attempt,'attempts',j.attempts,'warning',j.warning)::text
FROM travel_jobs j WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.id=j.message_id);
INSERT INTO storage_orphaned_message_metadata(source_table,message_id,row_data)
SELECT 'ews_mail_metadata',e.message_id,json_build_object('message_id',e.message_id,'data',e.data)::text
FROM ews_mail_metadata e WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.id=e.message_id);
DELETE FROM travel_jobs WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.id=travel_jobs.message_id);
DELETE FROM ews_mail_metadata WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.id=ews_mail_metadata.message_id);
-- +goose Down
-- Deliberately retain the quarantine and never reintroduce invalid references.
-- Restore original rows only after their parent messages have been recovered.
