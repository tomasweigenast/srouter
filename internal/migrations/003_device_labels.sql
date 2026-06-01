-- +goose Up
CREATE TABLE IF NOT EXISTS device_labels (
    mac   TEXT PRIMARY KEY,
    label TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS device_labels;
