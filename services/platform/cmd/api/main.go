// Command api is the platform's entry point: read the environment, wire the
// object graph, serve HTTP until the process is killed.
package main

import (
	"log/slog"
	"os"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("loading configuration", slog.Any("error", err))
		os.Exit(1)
	}

	handler, err := app.New(cfg.StaticDir)
	if err != nil {
		logger.Error("building the platform", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("platform listening", slog.Int("port", cfg.Port))

	if err := httpserver.Run(httpserver.NewServer(cfg.Port, handler)); err != nil {
		logger.Error("serving http", slog.Any("error", err))
		os.Exit(1)
	}
}
