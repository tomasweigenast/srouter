-- +goose Up
ALTER TABLE doh_config ADD COLUMN custom_dot_ip       TEXT NOT NULL DEFAULT '';
ALTER TABLE doh_config ADD COLUMN custom_dot_tls_name TEXT NOT NULL DEFAULT '';
ALTER TABLE doh_config ADD COLUMN custom_doh_server   TEXT NOT NULL DEFAULT '';

-- +goose Down
CREATE TABLE IF NOT EXISTS doh_config_new (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    enabled           INTEGER NOT NULL DEFAULT 0,
    provider          TEXT    NOT NULL DEFAULT 'cloudflare',
    fallback_to_plain INTEGER NOT NULL DEFAULT 1,
    stashed_upstreams TEXT    NOT NULL DEFAULT '1.1.1.1,8.8.8.8'
);
INSERT INTO doh_config_new SELECT id, enabled, provider, fallback_to_plain, stashed_upstreams FROM doh_config;
DROP TABLE doh_config;
ALTER TABLE doh_config_new RENAME TO doh_config;
