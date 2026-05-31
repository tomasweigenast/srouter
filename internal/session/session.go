package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	CookieName = "srouter_session"
	TTL        = 24 * time.Hour
)

type Session struct {
	ID        string
	Username  string
	ExpiresAt time.Time
}

type contextKey struct{}

var sessionKey = contextKey{}

func Create(db *sql.DB, username string) (Session, error) {
	s := Session{
		ID:        uuid.NewString(),
		Username:  username,
		ExpiresAt: time.Now().Add(TTL),
	}
	_, err := db.Exec(
		`INSERT INTO sessions (id, created_at, expires_at, username) VALUES (?, ?, ?, ?)`,
		s.ID, time.Now().Unix(), s.ExpiresAt.Unix(), s.Username,
	)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return s, nil
}

func Get(db *sql.DB, id string) (Session, bool, error) {
	var s Session
	var expiresAt int64
	err := db.QueryRow(
		`SELECT id, username, expires_at FROM sessions WHERE id = ? AND expires_at > ?`,
		id, time.Now().Unix(),
	).Scan(&s.ID, &s.Username, &expiresAt)
	if err == sql.ErrNoRows {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("get session: %w", err)
	}
	s.ExpiresAt = time.Unix(expiresAt, 0)
	return s, true, nil
}

func Delete(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func Cleanup(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("cleanup sessions: %w", err)
	}
	return nil
}

func CleanupLoop(db *sql.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		_ = Cleanup(db)
	}
}

func NewContext(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey).(Session)
	return s, ok
}
