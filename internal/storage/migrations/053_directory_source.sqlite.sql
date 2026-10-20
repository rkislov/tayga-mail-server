ALTER TABLE directory_groups ADD COLUMN external_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_directory_external ON directory_groups(domain_id,external_id) WHERE external_id<>'';
