-- +goose Up
CREATE TABLE IF NOT EXISTS domain_admin_domains (
    user_id TEXT NOT NULL,
    domain_id TEXT NOT NULL,
    PRIMARY KEY (user_id, domain_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
);
UPDATE users SET roles = REPLACE(roles, 'admin', 'global_admin')
 WHERE roles = 'admin' OR roles LIKE 'admin,%' OR roles LIKE '%,admin' OR roles LIKE '%,admin,%';

-- +goose Down
DROP TABLE IF EXISTS domain_admin_domains;
