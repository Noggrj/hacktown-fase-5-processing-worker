package logging

import (
	"log/slog"
	"os"
)

func New(service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h).With(slog.String("service", service))
}
