// Package config reads the platform's runtime configuration from the
// environment. Nothing else in the codebase is allowed to read an environment
// variable directly: this is the one seam, so a new setting has one place to
// be added and one place to be tested.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// defaultPort is used when ORCHESTRA_PORT is unset.
const defaultPort = 8080

// Config is the platform's runtime configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// StaticDir, when non-empty, is served at "/" as the built frontend.
	StaticDir string
}

// Load reads Config from the environment. ORCHESTRA_PORT defaults to 8080
// when unset; ORCHESTRA_STATIC_DIR defaults to empty, which means no static
// assets are served.
func Load() (Config, error) {
	cfg := Config{
		Port:      defaultPort,
		StaticDir: os.Getenv("ORCHESTRA_STATIC_DIR"),
	}

	raw, ok := os.LookupEnv("ORCHESTRA_PORT")
	if !ok || raw == "" {
		return cfg, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return Config{}, fmt.Errorf("ORCHESTRA_PORT: %w", err)
	}

	cfg.Port = port

	return cfg, nil
}
