-- +goose Up
CREATE TABLE xmpp_omemo_owners (owner TEXT PRIMARY KEY);
CREATE TABLE xmpp_omemo_retired (
    owner TEXT NOT NULL,
    device_id INTEGER NOT NULL CHECK(device_id > 0 AND device_id <= 2147483647),
    retired_at TIMESTAMP NOT NULL,
    PRIMARY KEY(owner, device_id)
);
-- +goose Down
DROP TABLE xmpp_omemo_retired;
DROP TABLE xmpp_omemo_owners;
