-- +goose Up
CREATE TABLE IF NOT EXISTS device_blocks (
    mac        TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- +goose Down
DROP TABLE IF EXISTS device_blocks;
