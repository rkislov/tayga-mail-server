-- +goose Up
CREATE TABLE IF NOT EXISTS domain_admin_domains (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, domain_id)
);
UPDATE users SET roles = REPLACE(roles, 'admin', 'global_admin')
 WHERE roles = 'admin' OR roles LIKE 'admin,%' OR roles LIKE '%,admin' OR roles LIKE '%,admin,%';

-- +goose Down
DROP TABLE IF EXISTS domain_admin_domains;
