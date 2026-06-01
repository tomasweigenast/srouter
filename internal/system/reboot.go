package system

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// ScheduleDailyReboot stores a daily reboot time (e.g. "23:30").
func ScheduleDailyReboot(db *sql.DB, timeOfDay string) error {
	_, err := db.Exec(
		`INSERT INTO scheduled_reboot (id, time_of_day) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET time_of_day = excluded.time_of_day`,
		timeOfDay,
	)
	return err
}

// CancelScheduledReboot removes the daily reboot schedule.
func CancelScheduledReboot(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM scheduled_reboot WHERE id = 1`)
	return err
}

// GetScheduledReboot returns the configured daily reboot time (HH:MM) if set.
func GetScheduledReboot(db *sql.DB) (timeOfDay string, ok bool, err error) {
	err = db.QueryRow(`SELECT time_of_day FROM scheduled_reboot WHERE id = 1`).Scan(&timeOfDay)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return timeOfDay, err == nil, err
}

func ExecuteReboot() error {
	if err := exec.Command("reboot").Run(); err != nil {
		return fmt.Errorf("reboot: %w", err)
	}
	return nil
}

// RebootWatchLoop fires a reboot every day at the configured HH:MM.
// It ticks every 30 seconds and uses a "last fired" date to avoid double-triggering.
func RebootWatchLoop(db *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var lastFired string // "YYYY-MM-DD" of last reboot trigger
	for now := range ticker.C {
		tod, ok, err := GetScheduledReboot(db)
		if err != nil || !ok {
			continue
		}
		today := now.Format("2006-01-02")
		current := now.Format("15:04")
		if current == tod && lastFired != today {
			lastFired = today
			slog.Info("executing daily scheduled reboot", "time", tod)
			ExecuteReboot()
			return
		}
	}
}
