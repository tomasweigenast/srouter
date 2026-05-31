package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/migrations"
)

var logger = logging.GetLogger("db")

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(&gooseSlogAdapter{logging.GetLogger("migrations")})

	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	if err := goose.Up(db, "."); err != nil {
		return err
	}

	logger.Info("migrations applied")
	return nil
}

// gooseSlogAdapter adapts *slog.Logger to the goose.Logger interface.
type gooseSlogAdapter struct{ l *slog.Logger }

func (a *gooseSlogAdapter) Fatalf(format string, v ...any) {
	a.l.Error(fmt.Sprintf(format, v...))
}

func (a *gooseSlogAdapter) Printf(format string, v ...any) {
	a.l.Info(fmt.Sprintf(format, v...))
}
