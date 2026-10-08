-- +goose Up
CREATE TABLE IF NOT EXISTS calendar_resources (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL UNIQUE,
    local_part TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL DEFAULT 'room',
    capacity INTEGER NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT '',
    auto_accept INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain_id, local_part)
);

CREATE INDEX IF NOT EXISTS idx_cal_resources_domain ON calendar_resources(domain_id);
CREATE INDEX IF NOT EXISTS idx_cal_resources_tenant ON calendar_resources(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cal_resources_user ON calendar_resources(user_id);

-- +goose Down
DROP TABLE IF EXISTS calendar_resources;
