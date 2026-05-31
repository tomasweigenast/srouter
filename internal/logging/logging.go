package logging

import (
	"log/slog"
	"os"
)

var logger = slog.New(
	slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}),
)

func init() {
	slog.SetDefault(logger)
}

func GetLogger(name string) *slog.Logger {
	return logger.With("source", name)
}
