-- +goose Up
CREATE TABLE IF NOT EXISTS scheduled_reboot (
    id        INTEGER PRIMARY KEY CHECK (id = 1),
    reboot_at INTEGER NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS scheduled_reboot;
