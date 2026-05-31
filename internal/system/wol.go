package system

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

type WoLDevice struct {
	ID        int64
	Name      string
	MAC       string
	IP        string
	CreatedAt time.Time
}

func ListWoLDevices(db *sql.DB) ([]WoLDevice, error) {
	rows, err := db.Query(`SELECT id, name, mac, COALESCE(ip,''), created_at FROM wol_devices ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list wol devices: %w", err)
	}
	defer rows.Close()

	var devices []WoLDevice
	for rows.Next() {
		var d WoLDevice
		var createdAt int64
		if err := rows.Scan(&d.ID, &d.Name, &d.MAC, &d.IP, &createdAt); err != nil {
			return nil, fmt.Errorf("scan wol device: %w", err)
		}
		d.CreatedAt = time.Unix(createdAt, 0)
		devices = append(devices, d)
	}
	return devices, nil
}

func AddWoLDevice(db *sql.DB, d WoLDevice) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO wol_devices (name, mac, ip) VALUES (?, ?, ?)`,
		d.Name, d.MAC, d.IP,
	)
	if err != nil {
		return 0, fmt.Errorf("add wol device: %w", err)
	}
	return res.LastInsertId()
}

func DeleteWoLDevice(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM wol_devices WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete wol device: %w", err)
	}
	return nil
}

func SendMagicPacket(mac string) error {
	// Normalize MAC — strip separators
	clean := strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", "")
	if len(clean) != 12 {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}
	macBytes, err := hex.DecodeString(clean)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %w", err)
	}

	// Build 102-byte magic packet: 6×0xFF + 16×MAC
	packet := make([]byte, 102)
	for i := range 6 {
		packet[i] = 0xFF
	}
	for i := range 16 {
		copy(packet[6+i*6:], macBytes)
	}

	conn, err := net.Dial("udp", "255.255.255.255:9")
	if err != nil {
		return fmt.Errorf("dial broadcast: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("send magic packet: %w", err)
	}
	return nil
}
