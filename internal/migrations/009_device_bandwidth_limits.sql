-- +goose Up

-- device_bandwidth_limits: per-device download bandwidth cap in Mbps.
-- Download limits are applied to egress traffic on the LAN interface (traffic
-- flowing from the router to the device). Absence of a row means unlimited.
-- Only preset values are accepted by the application layer; the CHECK only
-- enforces a positive integer.
CREATE TABLE IF NOT EXISTS device_bandwidth_limits (
    mac            TEXT PRIMARY KEY,
    download_mbps  INTEGER NOT NULL CHECK (download_mbps > 0),
    created_at     INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at     INTEGER NOT NULL DEFAULT (unixepoch())
);

-- +goose Down

DROP TABLE IF EXISTS device_bandwidth_limits;
