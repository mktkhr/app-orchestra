// Command api is the inventory service's entry point: read the environment,
// wire the object graph, serve HTTP until the process is killed.
package main

import (
	"log/slog"
	"os"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/infra/config"
	"github.com/mktkhr/app-orchestra/services/inventory/internal/infra/httpserver"
	"github.com/mktkhr/app-orchestra/services/inventory/pkg/app"
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
		logger.Error("building the inventory service", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("inventory listening", slog.Int("port", cfg.Port))

	if err := httpserver.Run(httpserver.NewServer(cfg.Port, handler)); err != nil {
		logger.Error("serving http", slog.Any("error", err))
		os.Exit(1)
	}
}
