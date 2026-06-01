package system

import (
	"database/sql"
	"errors"
)

type DeviceLabel struct {
	MAC   string
	Label string
}

func GetDeviceLabel(db *sql.DB, mac string) (string, error) {
	var label string
	err := db.QueryRow(`SELECT label FROM device_labels WHERE mac = ?`, mac).Scan(&label)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return label, err
}

func SetDeviceLabel(db *sql.DB, mac, label string) error {
	_, err := db.Exec(
		`INSERT INTO device_labels (mac, label) VALUES (?, ?)
		 ON CONFLICT(mac) DO UPDATE SET label = excluded.label`,
		mac, label,
	)
	return err
}

func DeleteDeviceLabel(db *sql.DB, mac string) error {
	_, err := db.Exec(`DELETE FROM device_labels WHERE mac = ?`, mac)
	return err
}

func ListDeviceLabels(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT mac, label FROM device_labels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	labels := map[string]string{}
	for rows.Next() {
		var mac, label string
		if err := rows.Scan(&mac, &label); err != nil {
			return nil, err
		}
		labels[mac] = label
	}
	return labels, rows.Err()
}
