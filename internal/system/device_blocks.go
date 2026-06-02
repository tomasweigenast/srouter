package system

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func IsDeviceBlocked(db *sql.DB, mac string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(1) FROM device_blocks WHERE mac = ?`, mac).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return count > 0, err
}

// BlockDevice saves the block to the DB, inserts the iptables rule at the top
// of the FORWARD chain (before any ACCEPT rules), and updates the firewall script.
func BlockDevice(db *sql.DB, mac string) error {
	if _, err := db.Exec(
		`INSERT INTO device_blocks (mac) VALUES (?) ON CONFLICT(mac) DO NOTHING`, mac,
	); err != nil {
		return err
	}
	// -I FORWARD 1 inserts at position 1 so it fires before any ACCEPT rules
	exec.Command("iptables", "-I", "FORWARD", "1",
		"-m", "mac", "--mac-source", mac, "-j", "DROP").Run()
	return writeBlocksScript(db)
}

// UnblockDevice removes the block from the DB, deletes the iptables rule, and
// updates the firewall script.
func UnblockDevice(db *sql.DB, mac string) error {
	if _, err := db.Exec(`DELETE FROM device_blocks WHERE mac = ?`, mac); err != nil {
		return err
	}
	// Ignore error — rule may not exist if firewall was reloaded without it
	exec.Command("iptables", "-D", "FORWARD",
		"-m", "mac", "--mac-source", mac, "-j", "DROP").Run()
	return writeBlocksScript(db)
}

// writeBlocksScript rewrites /etc/firewall.d/70-device-blocks.sh so blocks
// survive a firewall restart. Uses -I FORWARD 1 to stay ahead of ACCEPT rules.
func writeBlocksScript(db *sql.DB) error {
	macs, err := ListBlockedMACs(db)
	if err != nil {
		return fmt.Errorf("list blocked macs: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# Managed by srouter — do not edit manually\n")
	for mac := range macs {
		// -I FORWARD 1 inserts before any ACCEPT rules added by earlier scripts
		fmt.Fprintf(&sb, "iptables -I FORWARD 1 -m mac --mac-source %s -j DROP\n", mac)
	}

	if err := os.MkdirAll("/etc/firewall.d", 0755); err != nil {
		return fmt.Errorf("mkdir firewall.d: %w", err)
	}
	return os.WriteFile(blocksScript, []byte(sb.String()), 0750)
}
