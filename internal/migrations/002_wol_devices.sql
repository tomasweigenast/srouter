-- +goose Up

-- wol_devices: LAN devices saved for Wake-on-LAN (WoL).
-- The dashboard sends a UDP magic packet to 255.255.255.255:9 containing
-- the device's MAC address, which tells the NIC to power on the host.
-- Requires WoL to be enabled in the device's BIOS/UEFI firmware.
-- ip is optional — it is only stored as a human hint for display; the
-- actual magic packet is sent to the broadcast address, not the IP.
CREATE TABLE wol_devices (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,              -- human-readable label, e.g. "Desktop PC"
    mac        TEXT    NOT NULL UNIQUE,       -- colon-separated, e.g. aa:bb:cc:dd:ee:ff
    ip         TEXT,                          -- optional display hint, e.g. 192.168.0.50
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- +goose Down

DROP TABLE IF EXISTS wol_devices;
