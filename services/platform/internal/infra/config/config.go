// Package config reads the platform's runtime configuration from the
// environment. Nothing else in the codebase is allowed to read an environment
// variable directly: this is the one seam, so a new setting has one place to
// be added and one place to be tested.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// defaultPort is used when ORCHESTRA_PORT is unset.
const defaultPort = 8080

// ErrInvalidServiceEntry is wrapped into the error returned when
// ORCHESTRA_SERVICES contains an entry that is not "name=url".
var ErrInvalidServiceEntry = errors.New("invalid ORCHESTRA_SERVICES entry, want name=url")

// Service is one entry of ORCHESTRA_SERVICES: a service's name and the base
// URL its /openapi.yaml is fetched from.
type Service struct {
	Name string
	URL  string
}

// Answer is one entry of PlanFixture.Answers, decoded from
// ORCHESTRA_PLAN_FIXTURES. Mirrors pkg/app.Answer.
type Answer struct {
	Param string `json:"param"`
	Value string `json:"value"`
}

// PlanFixture is one entry of ORCHESTRA_PLAN_FIXTURES, decoded straight
// into the shape pkg/app.PlanFixture takes - see that type's doc comment
// for what each field means.
type PlanFixture struct {
	Query   string   `json:"query"`
	Answers []Answer `json:"answers"`

	Ask      bool   `json:"ask"`
	Question string `json:"question"`
	Param    string `json:"param"`

	Service     string         `json:"service"`
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
}

// Config is the platform's runtime configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// StaticDir, when non-empty, is served at "/" as the built frontend.
	StaticDir string
	// Services lists the microservices the catalogue is built from, read
	// from ORCHESTRA_SERVICES.
	Services []Service
	// LLMBaseURL is the OpenAI-compatible endpoint the tool-calling planner
	// (internal/adapter/planner/toolcall) talks to, read from
	// ORCHESTRA_LLM_BASE_URL. Empty means no real planner is configured:
	// pkg/app falls back to the stub planner in that case.
	LLMBaseURL string
	// LLMAPIKey is sent as that endpoint's bearer token, read from
	// ORCHESTRA_LLM_API_KEY. A local runtime such as llama-swap does not
	// check it, so it may be left empty.
	LLMAPIKey string
	// LLMModel is the model named in every request to that endpoint, read
	// from ORCHESTRA_LLM_MODEL - naming a different model is how a
	// different backend gets used behind a router such as llama-swap (D5,
	// docs/specs/orchestration.md).
	LLMModel string
	// PlanFixtures configures the stub planner's table when LLMBaseURL is
	// empty, read as a JSON array from ORCHESTRA_PLAN_FIXTURES. Production
	// never sets this - an operator sets ORCHESTRA_LLM_BASE_URL instead,
	// which makes pkg/app ignore PlanFixtures entirely (see
	// pkg/app.newPlanner). It exists only so a process started from the
	// built binary, such as e2e/src/orchestration.test.ts or
	// e2e/browser/chat.spec.ts, can drive the stub planner without an
	// in-process Go test's access to pkg/app.Config - `make check` must
	// never call a real LLM, and this is how the built product is
	// exercised without one.
	PlanFixtures []PlanFixture
}

// Load reads Config from the environment. ORCHESTRA_PORT defaults to 8080
// when unset; ORCHESTRA_STATIC_DIR defaults to empty, which means no static
// assets are served. ORCHESTRA_SERVICES defaults to empty, which means no
// service is configured.
func Load() (Config, error) {
	cfg := Config{
		Port:       defaultPort,
		StaticDir:  os.Getenv("ORCHESTRA_STATIC_DIR"),
		LLMBaseURL: os.Getenv("ORCHESTRA_LLM_BASE_URL"),
		LLMAPIKey:  os.Getenv("ORCHESTRA_LLM_API_KEY"),
		LLMModel:   os.Getenv("ORCHESTRA_LLM_MODEL"),
	}

	services, err := parseServices(os.Getenv("ORCHESTRA_SERVICES"))
	if err != nil {
		return Config{}, err
	}

	cfg.Services = services

	fixtures, err := parsePlanFixtures(os.Getenv("ORCHESTRA_PLAN_FIXTURES"))
	if err != nil {
		return Config{}, err
	}

	cfg.PlanFixtures = fixtures

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

// parseServices reads ORCHESTRA_SERVICES: a comma-separated list of
// "name=url" pairs, e.g.
// "inventory=http://localhost:8081,attendance=http://localhost:8082". An
// empty string yields no services rather than an error, so the platform
// still starts (with an empty catalogue) when nothing is configured.
func parseServices(raw string) ([]Service, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	entries := strings.Split(raw, ",")
	services := make([]Service, 0, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		name, url, ok := strings.Cut(entry, "=")
		if !ok || name == "" || url == "" {
			return nil, fmt.Errorf("%w: %q", ErrInvalidServiceEntry, entry)
		}

		services = append(services, Service{Name: name, URL: url})
	}

	return services, nil
}

// parsePlanFixtures reads ORCHESTRA_PLAN_FIXTURES: a JSON array of
// PlanFixture, or empty for none. See PlanFixture's doc comment for why
// this exists.
func parsePlanFixtures(raw string) ([]PlanFixture, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var fixtures []PlanFixture
	if err := json.Unmarshal([]byte(raw), &fixtures); err != nil {
		return nil, fmt.Errorf("ORCHESTRA_PLAN_FIXTURES: %w", err)
	}

	return fixtures, nil
}
