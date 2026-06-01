-- +goose Up
CREATE TABLE IF NOT EXISTS doh_config (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    enabled           INTEGER NOT NULL DEFAULT 0,
    provider          TEXT    NOT NULL DEFAULT 'cloudflare',
    fallback_to_plain INTEGER NOT NULL DEFAULT 1,
    stashed_upstreams TEXT    NOT NULL DEFAULT '1.1.1.1,8.8.8.8'
);
INSERT OR IGNORE INTO doh_config (id) VALUES (1);

-- +goose Down
DROP TABLE IF EXISTS doh_config;
