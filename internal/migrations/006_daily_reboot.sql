-- +goose Up
DROP TABLE IF EXISTS scheduled_reboot;
CREATE TABLE IF NOT EXISTS scheduled_reboot (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    time_of_day  TEXT NOT NULL  -- "HH:MM" in 24h format
);

-- +goose Down
DROP TABLE IF EXISTS scheduled_reboot;
