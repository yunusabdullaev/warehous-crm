package logger

import (
	"log/slog"
	"os"
)

// Init initializes the global structured JSON logger compatible with Loki/Grafana.
func Init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
}
