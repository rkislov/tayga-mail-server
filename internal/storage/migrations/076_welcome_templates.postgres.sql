-- +goose Up
CREATE TABLE welcome_templates (
    kind TEXT NOT NULL CHECK (kind IN ('tenants', 'domains')),
    scope_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    domain_id TEXT REFERENCES domains(id) ON DELETE CASCADE,
    generation BIGINT NOT NULL CHECK (generation > 0),
    snapshot TEXT NOT NULL,
    PRIMARY KEY (kind, scope_id),
    CHECK ((kind = 'tenants' AND domain_id IS NULL AND scope_id = tenant_id)
        OR (kind = 'domains' AND domain_id IS NOT NULL AND scope_id = domain_id))
);
CREATE INDEX idx_welcome_tenant ON welcome_templates(tenant_id);
-- +goose Down
DROP TABLE welcome_templates;
