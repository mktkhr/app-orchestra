// Command api is the attendance service's entry point: read the environment,
// wire the object graph, serve HTTP until the process is killed.
package main

import (
	"log/slog"
	"os"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/infra/config"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/infra/httpserver"
	"github.com/mktkhr/app-orchestra/services/attendance/pkg/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("loading configuration", slog.Any("error", err))
		os.Exit(1)
	}

	handler, err := app.New()
	if err != nil {
		logger.Error("building the attendance service", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("attendance listening", slog.Int("port", cfg.Port))

	if err := httpserver.Run(httpserver.NewServer(cfg.Port, handler)); err != nil {
		logger.Error("serving http", slog.Any("error", err))
		os.Exit(1)
	}
}
