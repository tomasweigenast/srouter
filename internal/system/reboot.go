package system

import (
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/tomasweigenast/srouter/internal/logging"
)

var rebootLogger = logging.GetLogger("reboot")

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

// RebootWatchLoop fires a reboot at the configured daily HH:MM.
// Send to resetCh (buffered, cap 1) after saving or canceling the schedule
// to wake the loop immediately instead of waiting for the current timer to expire.
func RebootWatchLoop(db *sql.DB, resetCh <-chan struct{}) {
	for {
		tod, ok, err := GetScheduledReboot(db)
		if err != nil || !ok {
			// No schedule — block until the user sets one.
			<-resetCh
			continue
		}

		timer := time.NewTimer(time.Until(nextOccurrence(tod)))
		select {
		case <-resetCh:
			timer.Stop()
			continue
		case <-timer.C:
		}

		rebootLogger.Info("executing daily scheduled reboot", "time", tod)
		ExecuteReboot()
		return
	}
}

// nextOccurrence returns the next wall-clock moment for a "HH:MM" time-of-day.
// If that time has already passed today, it returns tomorrow's occurrence.
func nextOccurrence(tod string) time.Time {
	now := time.Now()
	t, _ := time.ParseInLocation("15:04", tod, now.Location())
	next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
