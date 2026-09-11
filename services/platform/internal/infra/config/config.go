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

// ErrInvalidLLMMode is wrapped into the error returned when ORCHESTRA_LLM_MODE
// names anything other than LLMModeToolCall or LLMModeJSON. An unrecognised
// value fails startup rather than silently falling back to the default
// (docs/plans/orchestration.md, Task 11) - a typo'd mode should not quietly
// run the wrong planner.
var ErrInvalidLLMMode = errors.New("invalid ORCHESTRA_LLM_MODE, want toolcall or json")

// ErrMissingDBPath is returned when ORCHESTRA_DB_PATH is unset. Workspaces
// live in the SQLite file it names (docs/specs/workspaces.md, W3); a
// platform that started anyway would keep every workspace in a file
// nobody chose, and forget it silently on the next restart onto a
// different default. Failing startup is the honest alternative
// (docs/plans/workspaces.md, Task 0, Step 3).
var ErrMissingDBPath = errors.New("ORCHESTRA_DB_PATH is required")

// The two values ORCHESTRA_LLM_MODE accepts: which of the two
// usecase.Planner adapters (internal/adapter/planner/toolcall,
// internal/adapter/planner/jsonmode) pkg/app.newPlanner selects when an LLM
// base URL is configured. LLMModeToolCall is the default.
const (
	LLMModeToolCall = "toolcall"
	LLMModeJSON     = "json"
)

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
	// LLMMode selects which usecase.Planner adapter pkg/app.newPlanner
	// builds when LLMBaseURL is set: LLMModeToolCall (the default, used
	// when ORCHESTRA_LLM_MODE is unset) or LLMModeJSON, for a model that
	// cannot call tools (docs/plans/orchestration.md, Task 11).
	LLMMode string
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
	// DBPath is the SQLite file workspaces are kept in, read from
	// ORCHESTRA_DB_PATH. Required: see ErrMissingDBPath.
	DBPath string
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

	mode, err := parseLLMMode(os.Getenv("ORCHESTRA_LLM_MODE"))
	if err != nil {
		return Config{}, err
	}

	cfg.LLMMode = mode

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

	dbPath, ok := os.LookupEnv("ORCHESTRA_DB_PATH")
	if !ok || dbPath == "" {
		return Config{}, ErrMissingDBPath
	}

	cfg.DBPath = dbPath

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

// parseLLMMode reads ORCHESTRA_LLM_MODE: LLMModeToolCall when unset, or
// exactly LLMModeToolCall or LLMModeJSON otherwise - anything else fails
// startup instead of silently falling back to the default (Config.LLMMode's
// doc comment; docs/plans/orchestration.md, Task 11).
func parseLLMMode(raw string) (string, error) {
	if raw == "" {
		return LLMModeToolCall, nil
	}

	if raw != LLMModeToolCall && raw != LLMModeJSON {
		return "", fmt.Errorf("%w: %q", ErrInvalidLLMMode, raw)
	}

	return raw, nil
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
