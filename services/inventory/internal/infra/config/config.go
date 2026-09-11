// Package config reads the inventory service's runtime configuration from
// the environment. Nothing else in the codebase is allowed to read an
// environment variable directly: this is the one seam, so a new setting has
// one place to be added and one place to be tested.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// defaultPort is used when ORCHESTRA_PORT is unset. The platform uses 8080;
// this service uses 8081 so both can run at once during development.
const defaultPort = 8081

// Config is the inventory service's runtime configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
}

// Load reads Config from the environment. ORCHESTRA_PORT defaults to 8081
// when unset.
func Load() (Config, error) {
	cfg := Config{Port: defaultPort}

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
