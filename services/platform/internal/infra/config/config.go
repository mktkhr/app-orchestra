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
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/wording"
)

// defaultPort is used when ORCHESTRA_PORT is unset.
const defaultPort = 8080

// defaultContextTurns is used when ORCHESTRA_CONTEXT_TURNS is unset.
// Mirrors usecase.DefaultContextWindow - kept as its own constant rather
// than importing usecase here, so config keeps reading only bytes off the
// environment and pkg/app stays the one place that decides what a parsed
// value means to the usecase layer (see this package's own doc comment).
const defaultContextTurns = 8

// ErrInvalidServiceEntry is wrapped into the error returned when
// ORCHESTRA_SERVICES contains an entry that is not "name=url".
var ErrInvalidServiceEntry = errors.New("invalid ORCHESTRA_SERVICES entry, want name=url")

// ErrInvalidLLMMode is wrapped into the error returned when ORCHESTRA_LLM_MODE
// names anything other than LLMModeToolCall or LLMModeJSON. An unrecognised
// value fails startup rather than silently falling back to the default
// (docs/plans/orchestration.md, Task 11) - a typo'd mode should not quietly
// run the wrong planner.
var ErrInvalidLLMMode = errors.New("invalid ORCHESTRA_LLM_MODE, want toolcall or json")

// ErrInvalidPlannerWording is wrapped into the error returned when
// ORCHESTRA_PLANNER_WORDING names anything other than wording.Names(). An
// unrecognised name fails startup rather than silently falling back to
// wording.Default() - the same reasoning ErrInvalidLLMMode already
// applies to ORCHESTRA_LLM_MODE (docs/specs/wording.md, AC-Q-102).
var ErrInvalidPlannerWording = errors.New("invalid ORCHESTRA_PLANNER_WORDING")

// ErrInvalidPlannerThinking is wrapped into the error returned when
// ORCHESTRA_PLANNER_THINKING names anything other than "on" or "off" -
// the same reasoning ErrInvalidLLMMode already applies to ORCHESTRA_LLM_MODE.
var ErrInvalidPlannerThinking = errors.New("invalid ORCHESTRA_PLANNER_THINKING, want on or off")

// ErrInvalidPlannerRepeatPenalty is wrapped into the error returned when
// ORCHESTRA_PLANNER_REPEAT_PENALTY is set to something that does not parse
// as a float.
var ErrInvalidPlannerRepeatPenalty = errors.New("ORCHESTRA_PLANNER_REPEAT_PENALTY must be a number")

// ErrInvalidPlannerRepeatLastN is wrapped into the error returned when
// ORCHESTRA_PLANNER_REPEAT_LAST_N is set to something other than a
// positive integer.
var ErrInvalidPlannerRepeatLastN = errors.New("ORCHESTRA_PLANNER_REPEAT_LAST_N must be a positive integer")

// ErrInvalidPlannerStages is wrapped into the error returned when
// ORCHESTRA_PLANNER_STAGES is set to something other than "1" or "2" - the
// same reasoning ErrInvalidPlannerThinking already applies to
// ORCHESTRA_PLANNER_THINKING (docs/specs/staging.md, section 6).
var ErrInvalidPlannerStages = errors.New("ORCHESTRA_PLANNER_STAGES must be 1 or 2")

// ErrInvalidPlannerToday is wrapped into the error returned when
// ORCHESTRA_PLANNER_TODAY does not parse as a YYYY-MM-DD date - the same
// reasoning ErrInvalidPlannerThinking already applies to
// ORCHESTRA_PLANNER_THINKING: a typo'd date should fail startup, not
// silently fall back to the real clock.
var ErrInvalidPlannerToday = errors.New("ORCHESTRA_PLANNER_TODAY must be YYYY-MM-DD")

// ErrInvalidPicker is wrapped into the error returned when ORCHESTRA_PICKER
// names anything other than PickerLocal, PickerJev or PickerHybrid - the
// same reasoning ErrInvalidLLMMode already applies to ORCHESTRA_LLM_MODE.
var ErrInvalidPicker = errors.New("ORCHESTRA_PICKER must be local, jev or hybrid")

// ErrMissingJevAPIKey is returned when ORCHESTRA_PICKER is PickerJev or
// PickerHybrid, or ORCHESTRA_GATE is GateJev, but ORCHESTRA_JEV_API_KEY
// is unset or empty - internal/adapter/planner/jev has no route to Jev
// without one (PickerHybrid needs a jev.Picker for its own Jev half
// exactly as PickerJev does). The picker and the gate share one key; see
// config_gate.go's loadGate for the ORCHESTRA_GATE=jev half of this check.
var ErrMissingJevAPIKey = errors.New(
	"ORCHESTRA_JEV_API_KEY is required when ORCHESTRA_PICKER=jev, ORCHESTRA_PICKER=hybrid or ORCHESTRA_GATE=jev",
)

// ErrInvalidJevCriteria is wrapped into the error returned when
// ORCHESTRA_JEV_CRITERIA is set to something other than JevCriteriaV1 or
// JevCriteriaV2 - the same reasoning ErrInvalidPicker already applies to
// ORCHESTRA_PICKER.
var ErrInvalidJevCriteria = errors.New("ORCHESTRA_JEV_CRITERIA must be v1 or v2")

// ErrMissingDBPath is returned when ORCHESTRA_DB_PATH is unset. Workspaces
// live in the SQLite file it names (docs/specs/workspaces.md, W3); a
// platform that started anyway would keep every workspace in a file
// nobody chose, and forget it silently on the next restart onto a
// different default. Failing startup is the honest alternative
// (docs/plans/workspaces.md, Task 0, Step 3).
var ErrMissingDBPath = errors.New("ORCHESTRA_DB_PATH is required")

// ErrMissingAdminPassword is returned when ORCHESTRA_ADMIN_PASSWORD is
// unset or empty. The first admin account is seeded from it
// (docs/specs/auth.md, section 3); a default password would be a way of
// having no password at all while appearing to, the same reasoning
// ErrMissingDBPath already applies to ORCHESTRA_DB_PATH
// (docs/plans/auth.md, Task 0, Step 3).
var ErrMissingAdminPassword = errors.New("ORCHESTRA_ADMIN_PASSWORD is required")

// ErrNarrowingIncomplete is returned when exactly one or two of
// ORCHESTRA_NARROWING_EMBED_MODEL, ORCHESTRA_NARROWING_RERANK_MODEL and
// ORCHESTRA_NARROWING_K are set. Narrowing is configured as a group of
// three - the embedder and the reranker must agree with the K the
// catalogue was cut to - and a platform that started on two of the three
// would run a narrower with a name or a size it never actually asked for.
// This is the same reasoning ErrMissingDBPath already applies to a single
// required variable (docs/specs/shortlisting.md, section 5).
var ErrNarrowingIncomplete = errors.New(
	"ORCHESTRA_NARROWING_EMBED_MODEL, ORCHESTRA_NARROWING_RERANK_MODEL and ORCHESTRA_NARROWING_K must be set together, or not at all",
)

// ErrInvalidNarrowingK is returned when ORCHESTRA_NARROWING_K is set to
// something other than a positive integer, mirroring
// ErrInvalidContextTurns's own refusal to accept a non-positive window.
var ErrInvalidNarrowingK = errors.New("ORCHESTRA_NARROWING_K must be a positive integer")

// ErrInvalidContextTurns is returned when ORCHESTRA_CONTEXT_TURNS is set to
// something other than a positive integer. A window of zero or fewer turns
// is not a valid configuration to ask for explicitly - Orchestrator itself
// already treats a non-positive window as "keep nothing" (see
// truncateTurns, internal/usecase/orchestrator.go) - so a typo here fails
// startup rather than silently emptying every conversation.
var ErrInvalidContextTurns = errors.New("ORCHESTRA_CONTEXT_TURNS must be a positive integer")

// The two values ORCHESTRA_LLM_MODE accepts: which of the two
// usecase.Planner adapters (internal/adapter/planner/toolcall,
// internal/adapter/planner/jsonmode) pkg/app.newPlanner selects when an LLM
// base URL is configured. LLMModeToolCall is the default.
const (
	LLMModeToolCall = "toolcall"
	LLMModeJSON     = "json"
)

// PickerLocal, PickerJev and PickerHybrid are ORCHESTRA_PICKER's three
// accepted values: internal/adapter/planner/pick, internal/adapter/planner/jev
// and internal/adapter/planner/hybrid (Jev first, falling back to the
// local picker below its own confidence threshold - config_hybrid.go),
// respectively. PickerLocal is the default.
const (
	PickerLocal  = "local"
	PickerJev    = "jev"
	PickerHybrid = "hybrid"
)

// defaultJevBaseURL is used when ORCHESTRA_JEV_BASE_URL is unset.
const defaultJevBaseURL = "https://api.typesafe.ai"

// JevCriteriaV1 and JevCriteriaV2 are ORCHESTRA_JEV_CRITERIA's two
// accepted values: internal/adapter/planner/jev's original one-line-per-
// option criteria (v1), and the richer per-option object (`what`,
// `examples`, `not_for`) the Jev trial's second round measures (v2).
// JevCriteriaV1 is the default, so a deployment that never sets this
// variable sends exactly what it always has.
const (
	JevCriteriaV1 = "v1"
	JevCriteriaV2 = "v2"
)

// defaultJevCriteria is used when ORCHESTRA_JEV_CRITERIA is unset.
const defaultJevCriteria = JevCriteriaV1

// GateNone, GateJev and defaultJevGateThreshold live in config_gate.go
// (guard-filelen, harness/quality/filelen.sh - this file was already at
// its own 1000-line cap before the v3 Jev trial's gate existed), the
// same split app_picker_test.go's own doc comment describes for
// pkg/app's test files.

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

// TurnFixture is one entry of PlanFixture.Turns, decoded straight into the
// shape pkg/app.TurnFixture takes.
type TurnFixture struct {
	Service     string `json:"service"`
	OperationID string `json:"operationId"`
}

// Chart is PlanFixture.Chart, decoded straight into the shape
// pkg/app.Chart takes - see PlanFixture's doc comment for what it is for.
// A plain, JSON-decodable struct rather than domain.Chart: this package
// may not import internal/domain (see this file's own package doc
// comment and docs/plans/proposing.md's "Gap you must close first" - no
// other file under internal/infra/config imports it either), so the
// conversion to a typed domain.Chart happens in pkg/app, the same seam
// that already turns Args (a bare map[string]any here too) into whatever
// the usecase layer needs.
type Chart struct {
	Category string `json:"category"`
	Value    string `json:"value"`
	Kind     string `json:"kind"`
}

// Option is one entry of PlanFixture.Options, decoded from
// ORCHESTRA_PLAN_FIXTURES. Mirrors pkg/app.Option (added 2026-09-16
// alongside the ask_user degradation fix): only meaningful alongside Ask -
// see pkg/app.PlanFixture's doc comment for what it stands in for.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// PlanFixture is one entry of ORCHESTRA_PLAN_FIXTURES, decoded straight
// into the shape pkg/app.PlanFixture takes - see that type's doc comment
// for what each field means.
type PlanFixture struct {
	Query   string        `json:"query"`
	Answers []Answer      `json:"answers"`
	Turns   []TurnFixture `json:"turns"`

	Ask      bool     `json:"ask"`
	Question string   `json:"question"`
	Param    string   `json:"param"`
	Options  []Option `json:"options"`

	// Propose, when true, builds a DecisionProposal (usecase.DecisionProposal)
	// instead of the default DecisionCall - the fixture-table equivalent of
	// a propose_panel tool call (docs/specs/proposing.md, section 3). A
	// fixture is exactly one of Ask, Propose or plain-call: Propose is
	// checked first (see pkg/app.toDecision), so setting both Ask and
	// Propose on the same fixture just means Ask is never reached.
	Propose bool `json:"propose"`
	// Component, Chart and Title are propose_panel's own optional
	// arguments (docs/specs/proposing.md, section 3): each is the "model's
	// own value" a real propose_panel call would have given, left
	// zero/nil here to mean the model gave none - Orchestrator.propose
	// fills those in from the catalogue exactly as it does for the real
	// planner (section 4). Transform is deliberately not offered: the
	// plan's own chosen example (docs/plans/proposing.md, Task 2) is
	// chart-only, and a fixture field with no test exercising it is a
	// field nobody can tell still does anything.
	Component string `json:"component"`
	Chart     *Chart `json:"chart"`
	Title     string `json:"title"`

	Service     string         `json:"service"`
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
}

// SeedAccount is one entry of ORCHESTRA_SEED_ACCOUNTS, decoded straight
// into the shape pkg/app.SeedAccount takes - see that type's, and
// Config.SeedAccounts', doc comments for what this exists for.
type SeedAccount struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
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
	// PlannerWording selects the toolcall planner's named set of words
	// (internal/adapter/planner/wording.ByName), read from
	// ORCHESTRA_PLANNER_WORDING. Defaults to wording.Default().Name
	// ("v1") when unset - AC-Q-101: with it unset, every request to the
	// model is byte-identical to today's. Only the toolcall planner reads
	// this (pkg/app.newPlanner); the jsonmode planner is out of scope.
	PlannerWording string
	// PlannerThinking selects whether the toolcall planner leaves Qwen3.5's
	// thinking on (true) or turns it off (false, the default), read from
	// ORCHESTRA_PLANNER_THINKING ("on" or "off"; unset means false).
	// Measured 2026-09-16 (docs/specs/shortlisting.md): thinking off gives
	// correct@1 67 / correct@shown 71 at a mean 1377ms and never hits
	// max_tokens; thinking on gives 67/70 at a mean 6223ms. This is the
	// platform's default, overridable per request by PlanRequest.thinking
	// (docs/specs/shortlisting.md). Only the toolcall planner reads this -
	// see PlannerWording's own doc comment.
	PlannerThinking bool
	// PlannerRepeatPenalty is chat.Request.RepeatPenalty for every toolcall
	// planning request, read from ORCHESTRA_PLANNER_REPEAT_PENALTY. nil
	// (unset) sends nothing - today's behaviour.
	PlannerRepeatPenalty *float64
	// PlannerRepeatLastN is chat.Request.RepeatLastN, read from
	// ORCHESTRA_PLANNER_REPEAT_LAST_N. Defaults to 64 when unset, regardless
	// of PlannerRepeatPenalty - it has no effect unless PlannerRepeatPenalty is also set.
	PlannerRepeatLastN int
	// PlannerStages selects how many model calls the toolcall planner
	// makes to resolve an ordinary question, read from
	// ORCHESTRA_PLANNER_STAGES ("1" or "2"; unset means 2) - documented
	// beside ORCHESTRA_PLANNER_THINKING (docs/specs/staging.md, section
	// 6): 1 is the single call, byte-identical whether explicitly set to
	// "1" (AC-S-101); 2 is the pick-then-fill path
	// (usecase.WithStages(2), internal/usecase/orchestrator_staging.go,
	// S1) and is the default since 2026-09-16 (+12 correct@1, faster,
	// every eval case at baseline). Only pkg/app.build reads this, to
	// decide whether to also build a usecase.Picker (pick.New) and pass
	// usecase.WithPicker alongside usecase.WithStages.
	PlannerStages int
	// PlannerToday pins the planners' "today" (toolcall.WithClock,
	// jsonmode.WithClock) to local midnight on this date, read from
	// ORCHESTRA_PLANNER_TODAY (a YYYY-MM-DD date). nil (unset) means the
	// real clock - production's default. A measurement run pins this so
	// that two runs on different calendar days send byte-identical
	// requests: every planning call's user content starts with 「今日は
	// YYYY-MM-DD（曜）です。」 (see WithClock's own doc comment), so
	// without a fixed date, a near-tie eval or shortlist row can move day
	// to day for no reason the corpus itself changed.
	PlannerToday *time.Time
	// Picker selects which usecase.Picker implementation pkg/app.build
	// builds, read from ORCHESTRA_PICKER: PickerLocal (the default) or
	// PickerJev. Only meaningful when PlannerStages is 2 - the
	// pick-then-fill path (docs/specs/staging.md, S1) is the only one that
	// ever calls a usecase.Picker at all; under PlannerStages 1 this field
	// is read but never acted on.
	Picker string
	// JevAPIKey is sent as internal/adapter/planner/jev's bearer token,
	// read from ORCHESTRA_JEV_API_KEY. Required when Picker is PickerJev
	// (ErrMissingJevAPIKey); ignored otherwise, so a value left set after
	// switching Picker back to PickerLocal is inert rather than an error.
	JevAPIKey string
	// JevBaseURL is the base URL internal/adapter/planner/jev calls, read
	// from ORCHESTRA_JEV_BASE_URL. Defaults to defaultJevBaseURL
	// ("https://api.typesafe.ai") when unset.
	JevBaseURL string
	// JevCriteria selects which shape internal/adapter/planner/jev builds
	// each shortlist entry's criteria into, read from
	// ORCHESTRA_JEV_CRITERIA: JevCriteriaV1 (the default, one descriptive
	// line per option) or JevCriteriaV2 (a `what`/`examples`/`not_for`
	// object per option). Ignored when Picker is not PickerJev.
	JevCriteria string
	// JevObjectInstructions opts internal/adapter/planner/jev's "pick"
	// question instructions into the v5 trial's own object form
	// (docs/measurements/jev-v5.md) instead of the plain string v1/v2
	// always sent - the default again since v5's own per-variable
	// isolation measured the object form regressing
	// real-attendance-detail with no confirmed offsetting gain. Read from
	// ORCHESTRA_JEV_OBJECT_INSTRUCTIONS (any non-empty value means true);
	// turns still reach "state" unchanged either way. Ignored when Picker
	// is not PickerJev.
	JevObjectInstructions bool
	// HybridJevTimeout and HybridThreshold configure internal/adapter/planner/hybrid.Picker,
	// read from ORCHESTRA_HYBRID_JEV_TIMEOUT and ORCHESTRA_HYBRID_THRESHOLD
	// (config_hybrid.go). Ignored when Picker is not PickerHybrid.
	HybridJevTimeout time.Duration
	HybridThreshold  float64
	// Gate selects which usecase.Gate implementation pkg/app.build builds,
	// read from ORCHESTRA_GATE: GateNone (the default, no gate at all) or
	// GateJev (internal/adapter/planner/jev's "noul refusal gate",
	// docs/measurements/jev-picker-v3.md). Only meaningful when
	// PlannerStages is 2 - the same reasoning Picker's own doc comment
	// gives.
	Gate string
	// JevGateThreshold is the noul probability at or above which the jev
	// Gate judges a question impossible, read from
	// ORCHESTRA_JEV_GATE_THRESHOLD. Defaults to defaultJevGateThreshold
	// (0.7) when unset. Ignored when Gate is not GateJev.
	JevGateThreshold float64
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
	// AdminPassword seeds the first admin account, read from
	// ORCHESTRA_ADMIN_PASSWORD. Required: see ErrMissingAdminPassword.
	AdminPassword string
	// SecureCookie is the Secure attribute of the session cookie, read
	// from ORCHESTRA_SECURE_COOKIE and true unless that says "false".
	//
	// A browser decides where a cookie may go by the scheme it sees. Over
	// a link that is encrypted but not TLS - a Tailscale address, which is
	// how this is actually looked at - a Secure cookie is stored by
	// nobody, and signing in appears to work and then does not. Turning it
	// off says that the scheme is http and something other than TLS is
	// keeping the wire honest. Anything anybody else can reach wants TLS
	// and this left alone.
	SecureCookie bool
	// SeedAccounts configures pkg/app.Config.SeedAccounts, read as a JSON
	// array from ORCHESTRA_SEED_ACCOUNTS. Production never sets this - an
	// operator has no route to it, since docs/specs/auth.md section 8
	// keeps account creation out of the UI and API alike. It exists for
	// the same reason PlanFixtures does: a process started from the built
	// binary (e2e/src/auth.test.ts, e2e/browser/auth.spec.ts) needs a
	// non-admin account to sign in as, and has no in-process Go test's
	// access to pkg/app.Config to seed one through directly. Setting an
	// environment variable is not a network route - the exclusion this
	// mirrors is about not exposing account creation over HTTP, which this does not do.
	SeedAccounts []SeedAccount
	// ContextTurns is the number of turns of a conversation Orchestrator.Plan
	// keeps, oldest dropped first, read from ORCHESTRA_CONTEXT_TURNS
	// (docs/specs/context.md, section 6). Defaults to defaultContextTurns when unset.
	ContextTurns int
	// NarrowingEmbedModel names the embedding model llama-swap serves at /v1/embeddings, read from ORCHESTRA_NARROWING_EMBED_MODEL.
	// Empty means narrowing is off (docs/specs/shortlisting.md, H7) - see ErrNarrowingIncomplete for what a partial setting means.
	NarrowingEmbedModel string
	// NarrowingRerankModel names the reranking model llama-swap serves at /v1/rerank, read from ORCHESTRA_NARROWING_RERANK_MODEL.
	NarrowingRerankModel string
	// NarrowingK is how many endpoints the shortlist is cut to, read from ORCHESTRA_NARROWING_K.
	NarrowingK int
	// ServiceRouter fields: see config_service_router.go - unlike Picker/Gate, consulted regardless of PlannerStages.
	ServiceRouter          string
	ServiceRouterThreshold float64
	ServiceRouterCriteria  string
	// Fill fields: see config_fill.go - all off/unchanged by default.
	FillSkipEmpty                                   bool
	FillEnum, FillEnumUnsetWording, FillEnumRefusal string
	FillEnumThreshold                               float64
}

// Load reads Config from the environment. ORCHESTRA_PORT defaults to 8080
// when unset; ORCHESTRA_STATIC_DIR defaults to empty, which means no static
// assets are served. ORCHESTRA_SERVICES defaults to empty, which means no
// service is configured.
func Load() (Config, error) {
	cfg := Config{
		Port:         defaultPort,
		SecureCookie: os.Getenv("ORCHESTRA_SECURE_COOKIE") != "false",
		StaticDir:    os.Getenv("ORCHESTRA_STATIC_DIR"),
		LLMBaseURL:   os.Getenv("ORCHESTRA_LLM_BASE_URL"),
		LLMAPIKey:    os.Getenv("ORCHESTRA_LLM_API_KEY"),
		LLMModel:     os.Getenv("ORCHESTRA_LLM_MODEL"),
	}

	if err := loadLLM(&cfg); err != nil {
		return Config{}, err
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

	seedAccounts, err := parseSeedAccounts(os.Getenv("ORCHESTRA_SEED_ACCOUNTS"))
	if err != nil {
		return Config{}, err
	}

	cfg.SeedAccounts = seedAccounts

	contextTurns, err := parseContextTurns(os.Getenv("ORCHESTRA_CONTEXT_TURNS"))
	if err != nil {
		return Config{}, err
	}

	cfg.ContextTurns = contextTurns

	if narrowingErr := loadNarrowing(&cfg); narrowingErr != nil {
		return Config{}, narrowingErr
	}

	if pickerErr := loadPickerAndGate(&cfg); pickerErr != nil {
		return Config{}, pickerErr
	}

	dbPath, ok := os.LookupEnv("ORCHESTRA_DB_PATH")
	if !ok || dbPath == "" {
		return Config{}, ErrMissingDBPath
	}

	cfg.DBPath = dbPath

	adminPassword, ok := os.LookupEnv("ORCHESTRA_ADMIN_PASSWORD")
	if !ok || adminPassword == "" {
		return Config{}, ErrMissingAdminPassword
	}

	cfg.AdminPassword = adminPassword

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

// parsePlannerWording reads ORCHESTRA_PLANNER_WORDING: wording.Default().Name
// ("v1") when unset, or exactly one of wording.Names() otherwise - see
// ErrInvalidPlannerWording for why anything else fails startup instead of
// falling back to the default. The error names every known wording, so an
// operator who mistypes one sees the full list rather than having to go
// read the source (AC-Q-102).
func parsePlannerWording(raw string) (string, error) {
	if raw == "" {
		return wording.Default().Name, nil
	}

	if _, ok := wording.ByName(raw); !ok {
		return "", fmt.Errorf("%w: %q, want one of %s", ErrInvalidPlannerWording, raw, strings.Join(wording.Names(), ", "))
	}

	return raw, nil
}

// defaultPlannerRepeatLastN is used for ORCHESTRA_PLANNER_REPEAT_LAST_N
// when unset - see Config.PlannerRepeatLastN's own doc comment.
const defaultPlannerRepeatLastN = 64

// plannerThinkingOn and plannerThinkingOff are the two values
// ORCHESTRA_PLANNER_THINKING accepts.
const (
	plannerThinkingOn  = "on"
	plannerThinkingOff = "off"
)

// parsePlannerThinking reads ORCHESTRA_PLANNER_THINKING: false (thinking
// off, the default - measured 2026-09-16, docs/specs/shortlisting.md) when
// unset or "off", true when "on" - anything else fails startup rather than
// silently falling back to the default, the same reasoning parseLLMMode
// already applies.
func parsePlannerThinking(raw string) (bool, error) {
	switch raw {
	case "", plannerThinkingOff:
		return false, nil
	case plannerThinkingOn:
		return true, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrInvalidPlannerThinking, raw)
	}
}

// plannerRepeatPenalty is parsePlannerRepeatPenalty's result: value is
// only meaningful when set is true. A struct, not a nilable *float64 or a
// (float64, bool, error) triple: the former trips linting's nilnil rule
// (a function that can return (nil, nil) is ambiguous between "not found"
// and "found nil"), the latter its unnamedResult rule (two adjacent plain
// scalar results, float64 and bool, are easy to swap by position) -
// harness/quality/go/golangci.yml enables both, and this is the shape
// that satisfies each without silencing either.
type plannerRepeatPenalty struct {
	value float64
	set   bool
}

// parsePlannerRepeatPenalty reads ORCHESTRA_PLANNER_REPEAT_PENALTY: the
// zero plannerRepeatPenalty (send nothing) when unset, set with the
// parsed float otherwise.
func parsePlannerRepeatPenalty(raw string) (plannerRepeatPenalty, error) {
	if raw == "" {
		return plannerRepeatPenalty{}, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return plannerRepeatPenalty{}, fmt.Errorf("%w: %q", ErrInvalidPlannerRepeatPenalty, raw)
	}

	return plannerRepeatPenalty{value: value, set: true}, nil
}

// parsePlannerRepeatLastN reads ORCHESTRA_PLANNER_REPEAT_LAST_N:
// defaultPlannerRepeatLastN when unset, or the positive integer it names
// otherwise.
func parsePlannerRepeatLastN(raw string) (int, error) {
	if raw == "" {
		return defaultPlannerRepeatLastN, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidPlannerRepeatLastN, raw)
	}

	return n, nil
}

// plannerStagesOneValue/plannerStagesTwoValue are ORCHESTRA_PLANNER_STAGES'
// own two accepted values, and plannerStagesOne/plannerStagesTwo their
// parsed int form - named once so neither the string comparison nor the
// magic number (mnd, harness/quality/go/golangci.yml) is repeated
// unnamed.
const (
	plannerStagesOneValue = "1"
	plannerStagesTwoValue = "2"
	plannerStagesOne      = 1
	plannerStagesTwo      = 2
)

// parsePlannerStages reads ORCHESTRA_PLANNER_STAGES: plannerStagesTwo (2,
// the default since 2026-09-16, docs/specs/staging.md section 6) when
// unset or "2", plannerStagesOne (1, the single call, AC-S-101) when "1" -
// anything else fails startup rather than silently falling back to the
// default, the same reasoning parsePlannerThinking already applies to
// ORCHESTRA_PLANNER_THINKING.
func parsePlannerStages(raw string) (int, error) {
	switch raw {
	case "", plannerStagesTwoValue:
		return plannerStagesTwo, nil
	case plannerStagesOneValue:
		return plannerStagesOne, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidPlannerStages, raw)
	}
}

// plannerTodayLayout is the date layout ORCHESTRA_PLANNER_TODAY is parsed
// with - the same "YYYY-MM-DD" the date line itself renders (see
// toolcall.WithClock's own doc comment).
const plannerTodayLayout = "2006-01-02"

// plannerToday is parsePlannerToday's result: value is only meaningful
// when set is true. A struct, not a nilable *time.Time, for the same
// reason plannerRepeatPenalty is one rather than a *float64 - see its own
// doc comment.
type plannerToday struct {
	value time.Time
	set   bool
}

// parsePlannerToday reads ORCHESTRA_PLANNER_TODAY: the zero plannerToday
// (the real clock) when unset, set with local midnight on the named date
// otherwise - anything else fails startup rather than silently falling
// back to the real clock, the same reasoning parsePlannerThinking already
// applies to ORCHESTRA_PLANNER_THINKING.
func parsePlannerToday(raw string) (plannerToday, error) {
	if raw == "" {
		return plannerToday{}, nil
	}

	today, err := time.ParseInLocation(plannerTodayLayout, raw, time.Local)
	if err != nil {
		return plannerToday{}, fmt.Errorf("%w: %q", ErrInvalidPlannerToday, raw)
	}

	return plannerToday{value: today, set: true}, nil
}

// parseContextTurns reads ORCHESTRA_CONTEXT_TURNS: defaultContextTurns when
// unset or empty, or the positive integer it names otherwise. See
// ErrInvalidContextTurns for why anything else fails startup instead of
// falling back to the default.
func parseContextTurns(raw string) (int, error) {
	if raw == "" {
		return defaultContextTurns, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidContextTurns, raw)
	}

	return n, nil
}

// requiredNarrowingVars is how many of the three ORCHESTRA_NARROWING_*
// variables must be set together, named so parseNarrowing's "all or none"
// check isn't a bare magic number (mnd, harness/quality/go/golangci.yml).
const requiredNarrowingVars = 3

// narrowingConfig is parseNarrowing's own parsed result: either the zero
// value (narrowing off, H7) or every field filled in together - never a
// partial mix, which parseNarrowing itself refuses with
// ErrNarrowingIncomplete before this type is ever built with one.
type narrowingConfig struct {
	embedModel  string
	rerankModel string
	k           int
}

// loadNarrowing reads the three ORCHESTRA_NARROWING_* variables and
// applies them to cfg, isolating Load itself from both the os.Getenv
// calls and parseNarrowing's own validation (funlen,
// harness/quality/go/golangci.yml).
// loadLLM reads ORCHESTRA_LLM_MODE and ORCHESTRA_PLANNER_WORDING and
// applies them to cfg, isolating Load itself from both os.Getenv calls
// and their own validation (funlen, harness/quality/go/golangci.yml) -
// the same reason loadNarrowing exists.
func loadLLM(cfg *Config) error {
	mode, err := parseLLMMode(os.Getenv("ORCHESTRA_LLM_MODE"))
	if err != nil {
		return err
	}

	cfg.LLMMode = mode

	plannerWording, err := parsePlannerWording(os.Getenv("ORCHESTRA_PLANNER_WORDING"))
	if err != nil {
		return err
	}

	cfg.PlannerWording = plannerWording

	thinking, err := parsePlannerThinking(os.Getenv("ORCHESTRA_PLANNER_THINKING"))
	if err != nil {
		return err
	}

	cfg.PlannerThinking = thinking

	repeatPenalty, err := parsePlannerRepeatPenalty(os.Getenv("ORCHESTRA_PLANNER_REPEAT_PENALTY"))
	if err != nil {
		return err
	}

	if repeatPenalty.set {
		cfg.PlannerRepeatPenalty = &repeatPenalty.value
	}

	repeatLastN, err := parsePlannerRepeatLastN(os.Getenv("ORCHESTRA_PLANNER_REPEAT_LAST_N"))
	if err != nil {
		return err
	}

	cfg.PlannerRepeatLastN = repeatLastN

	stages, err := parsePlannerStages(os.Getenv("ORCHESTRA_PLANNER_STAGES"))
	if err != nil {
		return err
	}

	cfg.PlannerStages = stages

	today, err := parsePlannerToday(os.Getenv("ORCHESTRA_PLANNER_TODAY"))
	if err != nil {
		return err
	}

	if today.set {
		cfg.PlannerToday = &today.value
	}

	return nil
}

func loadNarrowing(cfg *Config) error {
	narrowing, err := parseNarrowing(
		os.Getenv("ORCHESTRA_NARROWING_EMBED_MODEL"), os.Getenv("ORCHESTRA_NARROWING_RERANK_MODEL"), os.Getenv("ORCHESTRA_NARROWING_K"),
	)
	if err != nil {
		return err
	}

	cfg.NarrowingEmbedModel = narrowing.embedModel
	cfg.NarrowingRerankModel = narrowing.rerankModel
	cfg.NarrowingK = narrowing.k

	return nil
}

// parseNarrowing reads the three ORCHESTRA_NARROWING_* variables: all
// three empty returns the zero narrowingConfig and no error (narrowing
// off, H7); all three set returns them parsed; one or two set is
// ErrNarrowingIncomplete (docs/specs/shortlisting.md, section 5) - the
// same "all or none" rule ErrMissingDBPath's own doc comment describes
// for a single variable, extended to a group of three that must agree
// with each other.
func parseNarrowing(embedModel, rerankModel, rawK string) (narrowingConfig, error) {
	set := 0
	for _, v := range []string{embedModel, rerankModel, rawK} {
		if v != "" {
			set++
		}
	}

	if set == 0 {
		return narrowingConfig{}, nil
	}

	if set != requiredNarrowingVars {
		return narrowingConfig{}, ErrNarrowingIncomplete
	}

	k, err := strconv.Atoi(rawK)
	if err != nil || k <= 0 {
		return narrowingConfig{}, fmt.Errorf("%w: %q", ErrInvalidNarrowingK, rawK)
	}

	return narrowingConfig{embedModel: embedModel, rerankModel: rerankModel, k: k}, nil
}

// parsePicker reads ORCHESTRA_PICKER: PickerLocal when unset, or exactly
// PickerLocal, PickerJev or PickerHybrid otherwise - anything else fails
// startup rather than silently falling back to the default, the same
// reasoning parseLLMMode already applies to ORCHESTRA_LLM_MODE.
func parsePicker(raw string) (string, error) {
	switch raw {
	case "":
		return PickerLocal, nil
	case PickerLocal, PickerJev, PickerHybrid:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidPicker, raw)
	}
}

// parseJevCriteria reads ORCHESTRA_JEV_CRITERIA: defaultJevCriteria when
// unset, or exactly JevCriteriaV1 or JevCriteriaV2 otherwise - the same
// reasoning parsePicker already applies to ORCHESTRA_PICKER.
func parseJevCriteria(raw string) (string, error) {
	switch raw {
	case "":
		return defaultJevCriteria, nil
	case JevCriteriaV1, JevCriteriaV2:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidJevCriteria, raw)
	}
}

// loadPicker reads ORCHESTRA_PICKER, ORCHESTRA_JEV_API_KEY,
// ORCHESTRA_JEV_BASE_URL and ORCHESTRA_JEV_CRITERIA and applies them to
// cfg, isolating Load from both the os.Getenv calls and their own
// validation (funlen, harness/quality/go/golangci.yml). ORCHESTRA_JEV_API_KEY
// is required exactly when the picker is PickerJev or PickerHybrid
// (ErrMissingJevAPIKey) - loadGate (config_gate.go), called right after
// this from Load, adds its own requiredness check for ORCHESTRA_GATE=jev
// over the same cfg.JevAPIKey this function sets. A value left set when
// none of those apply is read but never validated.
func loadPicker(cfg *Config) error {
	picker, err := parsePicker(os.Getenv("ORCHESTRA_PICKER"))
	if err != nil {
		return err
	}

	cfg.Picker = picker

	apiKey := os.Getenv("ORCHESTRA_JEV_API_KEY")
	if (picker == PickerJev || picker == PickerHybrid) && apiKey == "" {
		return ErrMissingJevAPIKey
	}

	cfg.JevAPIKey = apiKey

	baseURL := os.Getenv("ORCHESTRA_JEV_BASE_URL")
	if baseURL == "" {
		baseURL = defaultJevBaseURL
	}

	cfg.JevBaseURL = baseURL

	criteria, err := parseJevCriteria(os.Getenv("ORCHESTRA_JEV_CRITERIA"))
	if err != nil {
		return err
	}

	cfg.JevCriteria = criteria

	cfg.JevObjectInstructions = os.Getenv("ORCHESTRA_JEV_OBJECT_INSTRUCTIONS") != ""

	return nil
}

// loadPickerAndGate calls loadPicker, then loadHybrid (config_hybrid.go),
// then loadGate (config_gate.go), in that order - loadGate's own
// ORCHESTRA_JEV_API_KEY requiredness check reads cfg.JevAPIKey, set by
// loadPicker. One function, not three calls inlined into Load, keeps
// Load's own cyclomatic complexity under gocyclo's cap
// (harness/quality/go/golangci.yml), the same reasoning loadNarrowing
// and loadLLM already exist for.
func loadPickerAndGate(cfg *Config) error {
	if err := loadPicker(cfg); err != nil {
		return err
	}

	if err := loadHybrid(cfg); err != nil {
		return err
	}

	return loadGate(cfg)
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

// parseSeedAccounts reads ORCHESTRA_SEED_ACCOUNTS: a JSON array of
// SeedAccount, or empty for none. See Config.SeedAccounts' doc comment for
// why this exists.
func parseSeedAccounts(raw string) ([]SeedAccount, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var accounts []SeedAccount
	if err := json.Unmarshal([]byte(raw), &accounts); err != nil {
		return nil, fmt.Errorf("ORCHESTRA_SEED_ACCOUNTS: %w", err)
	}

	return accounts, nil
}
