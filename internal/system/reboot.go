package system

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

func ScheduleReboot(db *sql.DB, t time.Time) error {
	_, err := db.Exec(
		`INSERT INTO scheduled_reboot (id, reboot_at) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET reboot_at = excluded.reboot_at`,
		t.Unix(),
	)
	return err
}

func CancelScheduledReboot(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM scheduled_reboot WHERE id = 1`)
	return err
}

func GetScheduledReboot(db *sql.DB) (time.Time, bool, error) {
	var ts int64
	err := db.QueryRow(`SELECT reboot_at FROM scheduled_reboot WHERE id = 1`).Scan(&ts)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return time.Unix(ts, 0), true, nil
}

func ExecuteReboot() error {
	if err := exec.Command("reboot").Run(); err != nil {
		return fmt.Errorf("reboot: %w", err)
	}
	return nil
}

// RebootWatchLoop checks every 30 seconds whether a scheduled reboot is due.
func RebootWatchLoop(db *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		t, ok, err := GetScheduledReboot(db)
		if err != nil || !ok {
			continue
		}
		if time.Now().After(t) {
			slog.Info("executing scheduled reboot")
			CancelScheduledReboot(db)
			ExecuteReboot()
			return
		}
	}
}
