package system

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
)

const blocksScript = "/etc/firewall.d/70-device-blocks.sh"

func ListBlockedMACs(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT mac FROM device_blocks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var mac string
		if err := rows.Scan(&mac); err != nil {
			return nil, err
		}
		out[mac] = struct{}{}
	}
	return out, rows.Err()
}

func BlockDevice(db *sql.DB, mac string) error {
	_, err := db.Exec(
		`INSERT INTO device_blocks (mac) VALUES (?) ON CONFLICT(mac) DO NOTHING`,
		mac,
	)
	return err
}

func UnblockDevice(db *sql.DB, mac string) error {
	_, err := db.Exec(`DELETE FROM device_blocks WHERE mac = ?`, mac)
	return err
}

func IsDeviceBlocked(db *sql.DB, mac string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(1) FROM device_blocks WHERE mac = ?`, mac).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return count > 0, err
}

// RebuildBlocksScript rewrites the firewall blocks script from the DB and re-applies the firewall.
func RebuildBlocksScript(db *sql.DB) error {
	macs, err := ListBlockedMACs(db)
	if err != nil {
		return fmt.Errorf("list blocked macs: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# Managed by srouter — do not edit manually\n")
	for mac := range macs {
		sb.WriteString(fmt.Sprintf("# BLOCK: %s\n", mac))
		sb.WriteString(fmt.Sprintf("iptables -A FORWARD -m mac --mac-source %s -j DROP\n", mac))
	}

	if err := os.MkdirAll("/etc/firewall.d", 0755); err != nil {
		return fmt.Errorf("mkdir firewall.d: %w", err)
	}
	if err := os.WriteFile(blocksScript, []byte(sb.String()), 0755); err != nil {
		return fmt.Errorf("write blocks script: %w", err)
	}

	return ApplyFirewall()
}
