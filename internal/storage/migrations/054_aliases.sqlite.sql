CREATE TABLE domain_aliases (
 id TEXT PRIMARY KEY,
 domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
 name TEXT NOT NULL UNIQUE,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_domain_aliases_domain ON domain_aliases(domain_id);
