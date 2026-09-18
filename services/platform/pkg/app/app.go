// Package app is the platform's composition root: the only public package,
// and the only place that wires the object graph together.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/auth/local"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	invokerhttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/invoker/http"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/narrowing/llamaswap"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jsonmode"
	stubplanner "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/stub"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/wording"
	sqlitestore "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	specsourcehttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/specsource/http"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// Service names a running service: the catalogue is built from its
// /openapi.yaml at startup, and calls are made against its base URL at
// request time.
type Service struct {
	Name string
	URL  string
}

// Answer is one entry of PlanFixture.Answers: an answer to a previous ask,
// resubmitted alongside the original query. A local type, rather than
// usecase.Answer used directly on Config, so pkg/app's one public package
// does not leak an internal one through its own field types.
type Answer struct {
	Param string
	Value string
}

// Option is one entry of PlanFixture.Options: a candidate value and its
// label, mirroring domain.Option the way Answer mirrors usecase.Answer -
// a local type so pkg/app's one public package does not leak an internal
// one through its own field types. Only meaningful alongside Ask (see
// PlanFixture's doc comment): it stands in for a real ask_user tool call's
// own "options" argument (toolcall.decisionFromAskUser), so an e2e test can
// stub askDegrade's rule 2 (internal/usecase/orchestrator_ask.go, added
// 2026-09-16) - two or more options on a safe operation's non-enum,
// non-required param - without needing a real model to supply them.
type Option struct {
	Value string
	Label string
}

// TurnFixture is one entry of PlanFixture.Turns: the service and operation
// an earlier turn in the conversation resolved to. Only what
// internal/adapter/planner/stub.TurnsKey actually reads - a fixture keys a
// conversation by which service it was about, not by the wording of an
// earlier question (docs/plans/context.md, Task 4).
type TurnFixture struct {
	Service     string
	OperationID string
}

// Chart is PlanFixture.Chart: a propose fixture's own chart axes, mirroring
// config.Chart the way Answer mirrors config.Answer - see PlanFixture's
// doc comment for what it is for.
type Chart struct {
	Category string
	Value    string
	Kind     string
}

// PlanFixture is one entry of the stub planner's table
// (internal/adapter/planner/stub): a question - together with the answers,
// if any, it was resubmitted with, and the conversation, if any, it was
// asked alongside - mapped to the Decision it should produce. Used only
// when Config.LLM.BaseURL is empty (see newPlanner) - the running platform
// answers through the real, tool-calling planner
// (internal/adapter/planner/toolcall) once ORCHESTRA_LLM_BASE_URL is set.
//
// Ask, when true, builds a DecisionAsk carrying Question and Param, plus
// Service and OperationID naming the operation the ask stands in for -
// Args is unused for it. Service and OperationID matter for an ask
// fixture, not just a call one: they are how Orchestrator.ask finds the
// one endpoint Param belongs to, since a parameter name such as "status"
// is not unique across services.
//
// Options carries through to Decision.Options verbatim (added 2026-09-16
// alongside the ask_user degradation fix). It still cannot hand out an
// enum value the catalogue would not: when Param names a real catalogue
// enum, optionsForParam wins exactly as it does for the real planner and
// Options here is ignored, matching a real ask_user call's own Options
// argument (toolcall.decisionFromAskUser's doc comment - "Orchestrator.ask
// never trusts it" for the enum case). Only when Param has no catalogue
// enum at all does askDegrade ever look at Options (its rule 2), which is
// the one case a fixture can use this field to stand in for a model
// supplying two or more concrete candidates of its own.
//
// Turns lets the same Query resolve differently depending on what came
// before it - the fixture-table equivalent of a follow-up question phrased
// with no service name (AC-M-101). Empty means what it always meant: a
// question with no conversation before it.
type PlanFixture struct {
	Query   string
	Answers []Answer
	Turns   []TurnFixture

	Ask      bool
	Question string
	Param    string
	Options  []Option

	// Propose, when true, builds a usecase.Decision{Kind: DecisionProposal}
	// instead of the default DecisionCall - the stub-fixture equivalent of
	// a propose_panel tool call (docs/specs/proposing.md, section 3).
	// Checked before Ask in toDecision, so a fixture is exactly one of
	// Ask, Propose or plain-call.
	Propose bool
	// Component, Chart and Title mirror propose_panel's own optional
	// arguments, read the same way toolcall.decisionFromProposePanel reads
	// a real tool call's: each is the model's own value when given, left
	// zero/nil to mean the model gave none, so Orchestrator.propose's
	// catalogue fallback runs exactly as it does for the real planner.
	// See config.PlanFixture's doc comment for why Transform is not
	// offered here.
	Component string
	Chart     *Chart
	Title     string

	Service     string
	OperationID string
	Args        map[string]any
}

// LLM configures the tool-calling planner
// (internal/adapter/planner/toolcall) that talks to a real,
// OpenAI-compatible model. It mirrors config.Config's own
// LLMBaseURL/LLMAPIKey/LLMModel fields for the same reason Service mirrors
// config.Service: pkg/app's one public package should not force its own
// callers to depend on internal/infra/config's types.
type LLM struct {
	BaseURL string
	APIKey  string
	Model   string
	// Mode selects which usecase.Planner adapter newPlanner builds: empty or
	// ModeToolCall picks internal/adapter/planner/toolcall (so a Config
	// that never mentions Mode - every test in this package that predates
	// Task 11 - keeps today's behaviour unchanged), ModeJSON picks
	// internal/adapter/planner/jsonmode, for a model that cannot call tools
	// (docs/plans/orchestration.md, Task 11). Mirrors
	// config.Config.LLMMode's already-validated string the way Service
	// mirrors config.Service; New still rejects any other value itself
	// (ErrInvalidLLMMode) rather than trusting every caller to have gone
	// through config.Load's own validation first.
	Mode string
	// Wording selects the toolcall planner's named set of words
	// (internal/adapter/planner/wording.ByName), mirroring
	// config.Config.PlannerWording the way Mode mirrors LLMMode. Empty
	// means wording.Default() ("v1": today's text, byte for byte,
	// AC-Q-101). Only the toolcall planner reads this - the jsonmode
	// planner is out of scope (docs/plans/wording.md, Task 1, Step 5).
	Wording string
	// Thinking selects whether the toolcall planner leaves Qwen3.5's
	// thinking on or turns it off (toolcall.WithThinking), mirroring
	// config.Config.PlannerThinking. A pointer, not a bare bool: nil (the
	// zero value - every test in this package that predates this option)
	// must mean "thinking on", the same default toolcall.New itself
	// applies, and a bare bool's zero value (false) cannot say that.
	Thinking *bool
	// RepeatPenalty and RepeatLastN are toolcall.WithRepeatPenalty's
	// arguments, mirroring config.Config.PlannerRepeatPenalty/
	// PlannerRepeatLastN. RepeatPenalty nil (the zero value) means the
	// option is not applied at all - today's behaviour.
	RepeatPenalty *float64
	RepeatLastN   int
	// Stages mirrors config.Config.PlannerStages: 0 or 1 (every test and
	// caller that predates this subproject) is today's single call; 2
	// makes build also construct a pick.Picker over this same LLM and
	// pass usecase.WithPicker/WithStages(2) to NewOrchestrator
	// (docs/specs/staging.md, S1, S6).
	Stages int
	// Today mirrors config.Config.PlannerToday: nil (every test and
	// caller that predates this option) leaves both planners on the real
	// clock (toolcall.New/jsonmode.New's own default, time.Now); set,
	// newPlanner passes toolcall.WithClock/jsonmode.WithClock a clock
	// that always returns this instant, so every planning call's date
	// line is pinned regardless of which day the process actually runs
	// on (docs/specs/staging.md's own reasoning for a fixed measurement
	// date).
	Today *time.Time
}

// Narrowing configures the llama-swap-backed usecase.Narrower
// (internal/adapter/narrowing/llamaswap) that cuts the catalogue to a
// shortlist before the planner ever sees it (docs/specs/shortlisting.md,
// H1/H2). Empty EmbedModel means narrowing is off (H7) - see newNarrower -
// which is the zero value, so every test and caller that predates this
// subproject builds a Config that behaves exactly as it always has.
// internal/infra/config.Load enforces the "all three or none" rule this
// mirrors (config.ErrNarrowingIncomplete) before cmd/api ever populates
// this struct.
type Narrowing struct {
	// EmbedModel names the embedding model llama-swap serves at
	// /v1/embeddings (ORCHESTRA_NARROWING_EMBED_MODEL).
	EmbedModel string
	// RerankModel names the reranking model llama-swap serves at
	// /v1/rerank (ORCHESTRA_NARROWING_RERANK_MODEL).
	RerankModel string
	// K is how many endpoints the shortlist is cut to
	// (ORCHESTRA_NARROWING_K).
	K int
}

// The two non-empty values LLM.Mode accepts, mirroring
// internal/infra/config.LLMModeToolCall and LLMModeJSON.
const (
	ModeToolCall = "toolcall"
	ModeJSON     = "json"
)

// Picker configures which usecase.Picker implementation stagingOptions
// builds when LLM.Stages is 2 (docs/specs/staging.md, S1; unused under
// Stages 1 - staging never calls a usecase.Picker at all). Mirrors
// config.Config's own Picker/JevAPIKey/JevBaseURL fields the way LLM
// mirrors LLMBaseURL et al.
type Picker struct {
	// Name is "" or PickerLocal (internal/adapter/planner/pick, the
	// default - every test and caller that predates this subproject) or
	// PickerJev (internal/adapter/planner/jev, TypeSafe's hosted Jev
	// API).
	Name string
	// JevAPIKey is sent as internal/adapter/planner/jev's bearer token.
	// Required when Name is PickerJev (ErrMissingJevAPIKey).
	JevAPIKey string
	// JevBaseURL is the base URL internal/adapter/planner/jev calls.
	// cmd/api always sets it from config.Config.JevBaseURL, which
	// already defaults to "https://api.typesafe.ai" when
	// ORCHESTRA_JEV_BASE_URL is unset - New itself applies no further
	// default, the same way it trusts config.Load's own validation for
	// LLM.Mode and re-checks it anyway (ErrInvalidLLMMode).
	JevBaseURL string
	// JevCriteria selects the shape internal/adapter/planner/jev builds
	// each shortlist entry's criteria into: "" or JevCriteriaV1 (the
	// default, one descriptive line per option) or JevCriteriaV2 (a
	// `what`/`examples`/`not_for` object per option, the Jev trial's
	// second round). Ignored when Name is not PickerJev.
	JevCriteria string
	// JevObjectInstructions opts internal/adapter/planner/jev's "pick"
	// question instructions into the v5 trial's own object form instead
	// of the plain string v1/v2 always sent (the default) - kept behind
	// this switch since v5's own per-variable isolation measured the
	// object form regressing real-attendance-detail
	// (docs/measurements/jev-v5.md), mirroring
	// config.Config.JevObjectInstructions. turns still reach "state"
	// unchanged either way. Ignored when Name is not PickerJev.
	JevObjectInstructions bool
	// HybridJevTimeout and HybridThreshold configure
	// internal/adapter/planner/hybrid.Picker
	// (hybrid.WithJevTimeout/hybrid.WithThreshold), mirroring
	// config.Config.HybridJevTimeout/HybridThreshold. Zero means "use
	// hybrid's own default" for each (see hybrid.New's own doc comment).
	// Ignored when Name is not PickerHybrid.
	HybridJevTimeout time.Duration
	HybridThreshold  float64
}

// JevCriteriaV1 and JevCriteriaV2 are Picker.JevCriteria's two non-empty
// values, mirroring internal/infra/config.JevCriteriaV1/JevCriteriaV2.
const (
	JevCriteriaV1 = "v1"
	JevCriteriaV2 = "v2"
)

// PickerLocal, PickerJev and PickerHybrid are Picker.Name's three
// non-empty values, mirroring
// internal/infra/config.PickerLocal/PickerJev/PickerHybrid.
const (
	PickerLocal  = "local"
	PickerJev    = "jev"
	PickerHybrid = "hybrid"
)

// Gate configures which usecase.Gate implementation stagingOptions builds
// when LLM.Stages is 2 (the v3 Jev trial's own "noul refusal gate",
// docs/measurements/jev-picker-v3.md; unused under Stages 1, the same
// reasoning Picker's own doc comment gives). Mirrors config.Config's own
// Gate/JevAPIKey/JevBaseURL/JevGateThreshold fields the way Picker
// mirrors Picker/JevAPIKey et al.
type Gate struct {
	// Name is "" or GateNone (no gate at all - the default) or GateJev
	// (internal/adapter/planner/jev.Gate).
	Name string
	// JevAPIKey is sent as internal/adapter/planner/jev's bearer token.
	// Required when Name is GateJev (ErrMissingJevAPIKey) - the same key
	// Picker.JevAPIKey sends; there is no separate gate key.
	JevAPIKey string
	// JevBaseURL is the base URL internal/adapter/planner/jev calls,
	// mirroring Picker.JevBaseURL's own doc comment.
	JevBaseURL string
	// JevGateThreshold is the noul probability at or above which the jev
	// Gate judges a question impossible. cmd/api always sets it from
	// config.Config.JevGateThreshold, which already defaults to 0.7 when
	// ORCHESTRA_JEV_GATE_THRESHOLD is unset. 0 (Go's own float64 zero
	// value, and never a threshold a real deployment would choose - it
	// would judge every question impossible) is treated by newGate as
	// "not set": jev.WithGateThreshold is then never given, and
	// jev.NewGate's own default (0.7, the same number) applies - the
	// same "zero value means default" shape Picker.JevCriteria's own ""
	// already gives WithCriteria.
	JevGateThreshold float64
}

// GateNone and GateJev are Gate.Name's two non-empty values, mirroring
// internal/infra/config.GateNone/GateJev.
const (
	GateNone = "none"
	GateJev  = "jev"
)

// ServiceRouter configures which usecase.ServiceRouter implementation
// build consults in Plan, before o.narrower.Narrow and regardless of
// LLM.Stages (docs/measurements/jev-full-catalogue.md; DECISIONS.md
// 2026-09-18 - unlike Picker/Gate, this is not gated on staging at all).
// Mirrors config.Config's own ServiceRouter/JevAPIKey/JevBaseURL/
// ServiceRouterThreshold fields the way Gate mirrors Gate/JevAPIKey et al.
type ServiceRouter struct {
	// Name is "" or ServiceRouterNone (no router at all - the default) or
	// ServiceRouterJev (internal/adapter/planner/jev.ServiceRouter).
	Name string
	// JevAPIKey is sent as internal/adapter/planner/jev's bearer token.
	// Required when Name is ServiceRouterJev (ErrMissingJevAPIKey) - the
	// same key Picker.JevAPIKey and Gate.JevAPIKey send.
	JevAPIKey string
	// JevBaseURL is the base URL internal/adapter/planner/jev calls,
	// mirroring Picker.JevBaseURL's own doc comment.
	JevBaseURL string
	// JevCriteria selects the shape the router builds its "route"
	// question's criteria into, mirroring Picker.JevCriteria's own doc
	// comment (this file's own doc comment: so a deployment's router and
	// picker always describe each service in the same words).
	JevCriteria string
	// JevCriteriaForm selects which content the router builds each
	// service's own criterion from: "" or jev.RouteCriteriaNames (the
	// default, a handful of that service's own operation names) or
	// jev.RouteCriteriaOps (every one of that service's own operations).
	// cmd/api always sets it from config.Config.ServiceRouterCriteria,
	// which already defaults to "names" when
	// ORCHESTRA_SERVICE_ROUTER_CRITERIA is unset.
	JevCriteriaForm string
	// Threshold is the confidence a ServiceRoute must be at or above for
	// Plan to act on it. cmd/api always sets it from
	// config.Config.ServiceRouterThreshold, which already defaults to 0.5
	// when ORCHESTRA_SERVICE_ROUTER_THRESHOLD is unset.
	Threshold float64
}

// ServiceRouterNone and ServiceRouterJev are ServiceRouter.Name's two
// non-empty values, mirroring
// internal/infra/config.ServiceRouterNone/ServiceRouterJev.
const (
	ServiceRouterNone = "none"
	ServiceRouterJev  = "jev"
)

// ErrInvalidLLMMode is returned by New when Config.LLM.Mode is set to
// anything other than "" (the default), ModeToolCall or ModeJSON -
// mirroring internal/infra/config.ErrInvalidLLMMode's own refusal to fall
// back to the default silently.
var ErrInvalidLLMMode = errors.New("invalid LLM.Mode, want \"\", \"toolcall\" or \"json\"")

// ErrInvalidPlannerWording is returned by New when Config.LLM.Wording
// names anything other than "" (wording.Default()) or one of
// wording.Names() - mirroring ErrInvalidLLMMode's own refusal to fall
// back to the default silently. cmd/api never reaches this: it always
// goes through config.Load's own validation first
// (config.ErrInvalidPlannerWording) - this is the same defence-in-depth
// ErrInvalidLLMMode already provides for a caller that builds a Config
// directly.
var ErrInvalidPlannerWording = errors.New("invalid LLM.Wording")

// ErrInvalidPicker is returned by New when Config.Picker.Name is set to
// anything other than "" (PickerLocal), PickerLocal, PickerJev or
// PickerHybrid - mirroring ErrInvalidLLMMode's own defence-in-depth:
// cmd/api always goes through config.Load's own validation first
// (config.ErrInvalidPicker).
var ErrInvalidPicker = errors.New("invalid Picker.Name, want \"\", \"local\", \"jev\" or \"hybrid\"")

// ErrMissingJevAPIKey is returned by New when Config.Picker.Name is
// PickerJev or PickerHybrid but Config.Picker.JevAPIKey is empty -
// mirroring config.ErrMissingJevAPIKey the same way ErrInvalidPicker
// mirrors config.ErrInvalidPicker: PickerHybrid needs a real jev.Picker
// for its own Jev half exactly as PickerJev does. Also returned when
// Config.Gate.Name is GateJev and Config.Gate.JevAPIKey is empty, or
// Config.ServiceRouter.Name is ServiceRouterJev and
// Config.ServiceRouter.JevAPIKey is empty - the picker, the gate and the
// service router share one error and one required key.
var ErrMissingJevAPIKey = errors.New(
	"Picker.JevAPIKey is required when Picker.Name is \"jev\" or \"hybrid\", or Gate.JevAPIKey when Gate.Name is " +
		"\"jev\", or ServiceRouter.JevAPIKey when ServiceRouter.Name is \"jev\"",
)

// ErrInvalidGate is returned by New when Config.Gate.Name is set to
// anything other than "" (GateNone), GateNone or GateJev - mirroring
// ErrInvalidPicker's own defence-in-depth: cmd/api always goes through
// config.Load's own validation first (config.ErrInvalidGate).
var ErrInvalidGate = errors.New("invalid Gate.Name, want \"\", \"none\" or \"jev\"")

// ErrInvalidServiceRouter is returned by New when Config.ServiceRouter.Name
// is set to anything other than "" (ServiceRouterNone), ServiceRouterNone
// or ServiceRouterJev - mirroring ErrInvalidGate's own defence-in-depth:
// cmd/api always goes through config.Load's own validation first
// (config.ErrInvalidServiceRouter).
var ErrInvalidServiceRouter = errors.New("invalid ServiceRouter.Name, want \"\", \"none\" or \"jev\"")

// ErrMissingDBPath is returned by New when Config.DBPath is empty. There
// is no legitimate use of this platform without a database - workspaces
// and accounts both need one - so New refuses to build a handler at all,
// rather than falling back to the wide-open admin bypass
// httpserver.requireSession used to run every request through when built
// with a nil SessionUsers store (docs/specs/auth.md, section A6: "a check
// that can be forgotten will be forgotten" - the shipped binary was safe
// only because internal/infra/config.Load already refuses to start
// without ORCHESTRA_DB_PATH (config.ErrMissingDBPath), which said nothing
// about any other caller of New).
var ErrMissingDBPath = errors.New("DBPath is required")

// Config is everything New needs to wire the platform's object graph.
type Config struct {
	// StaticDir, when non-empty, is served at "/" as the built frontend.
	StaticDir string
	// Services is the catalogue's source: the platform fetches each
	// one's /openapi.yaml at startup and calls its operations at request
	// time. No service configured means an empty catalogue.
	Services []Service
	// LLM configures the real, tool-calling planner. An empty BaseURL -
	// production's default until an operator sets ORCHESTRA_LLM_BASE_URL -
	// falls back to the stub planner over PlanFixtures instead (see
	// newPlanner): `make check` must never call a real LLM
	// (docs/plans/orchestration.md, global constraints), and nothing that
	// builds a Config for a test sets this field.
	LLM LLM
	// PlanFixtures configures the stub planner, used only when LLM.BaseURL
	// is empty. Empty means every question comes back as ResultKindNone -
	// there is no longer a hard-coded demo table (DECISIONS.md,
	// 2026-09-11): once a real planner exists, answering with two
	// hard-coded Japanese questions is no longer the honest default.
	PlanFixtures []PlanFixture
	// DBPath is the SQLite file workspaces, accounts and sessions are all
	// kept in (docs/specs/workspaces.md, W3). Required: see
	// ErrMissingDBPath. cmd/api always sets it, because
	// internal/infra/config.Load refuses to start without
	// ORCHESTRA_DB_PATH (config.ErrMissingDBPath) - New enforces the same
	// rule itself, rather than trusting every other caller to have gone
	// through config.Load first.
	DBPath string
	// SecureCookie is the Secure attribute of the session cookie. See
	// internal/infra/config.Config.SecureCookie for what turning it off
	// means and when it is the right thing to do.
	SecureCookie bool
	// AdminPassword seeds the first admin account, once, in the file named
	// by DBPath (docs/specs/auth.md, section 3). Required whenever New is
	// called at all, since DBPath now always is too - cmd/api always sets
	// both together (internal/infra/config.Load refuses to start without
	// ORCHESTRA_ADMIN_PASSWORD either).
	AdminPassword string
	// SeedAccounts puts each account in place - by name, only when it does
	// not already exist - once the admin account above is seeded. Set
	// directly by a Go acceptance test that needs a non-admin account to
	// sign in as, or by cmd/api when ORCHESTRA_SEED_ACCOUNTS names one
	// (internal/infra/config.Config.SeedAccounts) - the latter exists only
	// so a process started from the built binary
	// (e2e/src/auth.test.ts, e2e/browser/auth.spec.ts) can do the same
	// thing a Go test does directly, since it never sees pkg/app.Config.
	// Either way this is not a second, softer way to create an account
	// over the wire: it is how a caller of pkg/app.New, or the operator
	// starting the process, puts one in place through the composition
	// root rather than through a route (docs/specs/auth.md, section 8:
	// account creation stays out of the UI and the API; setting an
	// environment variable before the process starts serving is neither).
	SeedAccounts []SeedAccount
	// ContextTurns is the number of turns of a conversation Plan keeps,
	// oldest dropped first (docs/specs/context.md, section 6). Zero - the
	// zero value, left unset by every test that does not care about the
	// window - falls back to usecase.DefaultContextWindow (see New): it is
	// not itself a valid window (usecase.WithContextWindow(0) would keep
	// nothing), so New must never pass a bare zero through unquestioned.
	ContextTurns int
	// Narrowing configures the shortlist stage. Its zero value
	// (EmbedModel == "") is narrowing off - see newNarrower.
	Narrowing Narrowing
	// Picker selects which usecase.Picker implementation stagingOptions
	// builds when LLM.Stages is 2. Its zero value (Name == "") is
	// PickerLocal - every test and caller that predates this subproject
	// keeps today's behaviour unchanged.
	Picker Picker
	// Gate selects which usecase.Gate implementation stagingOptions
	// builds when LLM.Stages is 2. Its zero value (Name == "") is
	// GateNone (no gate at all) - every test and caller that predates
	// this subproject keeps today's behaviour unchanged.
	Gate Gate
	// ServiceRouter selects which usecase.ServiceRouter implementation
	// build consults in Plan, before o.narrower.Narrow and regardless of
	// LLM.Stages. Its zero value (Name == "") is ServiceRouterNone (no
	// router at all) - every test and caller that predates this
	// subproject keeps today's behaviour unchanged.
	ServiceRouter ServiceRouter
	// Fill selects the fill-stage experiment's two arms (see app_fill.go).
	// Its zero value (SkipEmpty == false, Enum == "") is both arms off -
	// every test and caller that predates this subproject keeps today's
	// behaviour unchanged.
	Fill Fill
}

// SeedAccount is one account for New to put in place via SeedAccounts,
// before it ever serves a request - see that field's own doc comment.
type SeedAccount struct {
	Name     string
	Password string
	// Role is domain.RoleAdmin or domain.RoleUser, as a plain string for
	// the same reason LLM.Mode mirrors config.Config's own string rather
	// than an internal type: pkg/app's one public package should not force
	// a caller to import internal/domain just to spell a role.
	Role string
}

// adminUserName is the name the first admin account is seeded under
// (docs/specs/auth.md, section 3). Fixed rather than configurable: nothing
// asks for a different one, and a person signs in with it the same way
// every time.
const adminUserName = "admin"

// New builds the platform's http.Handler. Acceptance tests and cmd/api
// both go through this one function, so both exercise the same object
// graph; neither needs to see internal/, which is the point of pkg/app
// being the only public package.
//
// cfg is a pointer, not the value shown in the plan, because Config is 112
// bytes: golangci-lint's gocritic hugeParam check (part of the fixed
// harness policy, see harness/quality/go/golangci.yml) rejects passing it
// by value once LLM was added to it - the same reason domain.Endpoint and
// usecase.Decision take a pointer.
func New(cfg *Config) (http.Handler, error) {
	return build(cfg, httpserver.NewRouter)
}

// build takes the router constructor as a parameter so that the failure
// path - which httpserver.NewRouter cannot exercise with a valid, embedded
// spec - stays under test here.
func build(
	cfg *Config,
	newRouter func(openapi.StrictServerInterface, string, httpserver.SessionUsers) (http.Handler, error),
) (http.Handler, error) {
	if cfg.DBPath == "" {
		return nil, ErrMissingDBPath
	}

	catalog, err := specsourcehttp.New(toSpecSourceServices(cfg.Services), nil).Fetch(context.Background())
	if err != nil {
		return nil, fmt.Errorf("building the catalogue: %w", err)
	}

	// One *sql.DB for cfg.DBPath, shared by every store below - not one
	// each (docs/specs/storage.md, S1, AC-S-102): four separate
	// connections to the same file, each with its own single-connection
	// pool, is what let a workspace write and a session read collide as
	// "database is locked" under real concurrency (that spec, section 2).
	db, err := sqlitestore.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("opening the database: %w", err)
	}

	permissions := newPermissionStore(db)

	workspaceHandler := newWorkspaceHandler(db, catalog, permissions)

	authenticator, sessions, users, err := newAuth(db, cfg.AdminPassword)
	if err != nil {
		return nil, err
	}

	if seedErr := seedAccounts(context.Background(), users, cfg.SeedAccounts); seedErr != nil {
		return nil, seedErr
	}

	planner, err := newPlanner(cfg, catalog)
	if err != nil {
		return nil, err
	}

	narrowerOption, err := newNarrowerOption(cfg, catalog)
	if err != nil {
		return nil, err
	}

	invoker := invokerhttp.New(toInvokerServices(cfg.Services), nil)

	staging, err := stagingOptions(cfg)
	if err != nil {
		return nil, err
	}

	serviceRouterOption, err := newServiceRouterOption(cfg)
	if err != nil {
		return nil, err
	}

	orchestratorOptions := append(
		[]usecase.Option{contextWindowOption(cfg.ContextTurns), narrowerOption, serviceRouterOption}, staging...,
	)
	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker, permissions, orchestratorOptions...)
	adminUsecase := usecase.NewAdmin(users, permissions, catalog)
	catalogUsecase := usecase.NewCatalog(catalog, permissions)

	api := handler.NewAPI(
		handler.NewHealth(),
		handler.NewSession(authenticator, sessions, cfg.SecureCookie),
		handler.NewPlan(orchestrator),
		handler.NewInvoke(orchestrator),
		workspaceHandler,
		handler.NewUsers(adminUsecase),
		handler.NewCatalog(catalogUsecase),
	)

	router, err := newRouter(api, cfg.StaticDir, sessions)
	if err != nil {
		return nil, fmt.Errorf("building the router: %w", err)
	}

	return router, nil
}

// toSpecSourceServices adapts Config.Services to the shape
// internal/adapter/specsource/http takes. A small conversion rather than a
// shared type: that package may not depend on pkg/app (layer order), and
// pkg/app should not force its own Service type on it either.
func toSpecSourceServices(services []Service) []specsourcehttp.Service {
	out := make([]specsourcehttp.Service, 0, len(services))
	for _, s := range services {
		out = append(out, specsourcehttp.Service{Name: s.Name, URL: s.URL})
	}

	return out
}

// toInvokerServices adapts Config.Services to the shape
// internal/adapter/invoker/http takes. See toSpecSourceServices.
func toInvokerServices(services []Service) []invokerhttp.Service {
	out := make([]invokerhttp.Service, 0, len(services))
	for _, s := range services {
		out = append(out, invokerhttp.Service{Name: s.Name, URL: s.URL})
	}

	return out
}

// toStubTable converts PlanFixtures into the table
// internal/adapter/planner/stub takes: each fixture's query, answers and
// turns become its stub.Key, and Ask selects a DecisionAsk instead of the
// default DecisionCall.
func toStubTable(fixtures []PlanFixture) map[stubplanner.Key]usecase.Decision {
	table := make(map[stubplanner.Key]usecase.Decision, len(fixtures))

	for i := range fixtures {
		f := &fixtures[i]
		key := stubplanner.Key{
			Query:   f.Query,
			Answers: stubplanner.AnswersKey(toUsecaseAnswers(f.Answers)),
			Turns:   stubplanner.TurnsKey(toUsecaseTurns(f.Turns)),
		}
		table[key] = toDecision(f)
	}

	return table
}

// toUsecaseTurns adapts PlanFixture.Turns to the shape
// internal/adapter/planner/stub's TurnsKey takes. See toUsecaseAnswers.
func toUsecaseTurns(turns []TurnFixture) []usecase.Turn {
	out := make([]usecase.Turn, 0, len(turns))
	for _, t := range turns {
		out = append(out, usecase.Turn{Service: t.Service, OperationID: t.OperationID})
	}

	return out
}

// toDecision builds the Decision one PlanFixture produces: a proposal when
// Propose is set, an ask when Ask is set, a call otherwise - checked in
// that order, so a fixture is exactly one of the three (PlanFixture.Propose's
// doc comment). See PlanFixture's doc comment for what Options on an ask
// fixture can and cannot hand out.
func toDecision(f *PlanFixture) usecase.Decision {
	if f.Propose {
		return usecase.Decision{
			Kind:        usecase.DecisionProposal,
			Service:     f.Service,
			OperationID: f.OperationID,
			Args:        f.Args,
			Component:   domain.Component(f.Component),
			View:        toDomainView(f.Chart),
			Title:       f.Title,
		}
	}

	if f.Ask {
		return usecase.Decision{
			Kind:        usecase.DecisionAsk,
			Service:     f.Service,
			OperationID: f.OperationID,
			Question:    f.Question,
			Param:       f.Param,
			Options:     toDomainOptions(f.Options),
		}
	}

	return usecase.Decision{
		Kind:        usecase.DecisionCall,
		Service:     f.Service,
		OperationID: f.OperationID,
		Args:        f.Args,
	}
}

// toDomainView builds a *domain.View from a propose fixture's Chart, or
// nil when the fixture gave none - a proposal fixture with no chart must
// map to a nil View, not an empty one, so Orchestrator.propose's catalogue
// fallback (proposalView) exercises exactly as it does for the real
// planner (docs/specs/proposing.md, section 4). Transform is always nil
// here: see PlanFixture.Chart's doc comment for why this fixture shape
// offers no transform.
func toDomainView(chart *Chart) *domain.View {
	if chart == nil {
		return nil
	}

	return &domain.View{
		Chart: &domain.Chart{
			Category: chart.Category,
			Value:    chart.Value,
			Kind:     domain.ChartKind(chart.Kind),
		},
	}
}

// toDomainOptions adapts PlanFixture.Options to []domain.Option, the shape
// Decision.Options carries - nil, not an empty slice, for a fixture with no
// options at all, the same "absent, not empty" convention toUsecaseAnswers
// and toUsecaseTurns already follow for their own fixture slices.
func toDomainOptions(options []Option) []domain.Option {
	if len(options) == 0 {
		return nil
	}

	out := make([]domain.Option, len(options))
	for i, o := range options {
		out[i] = domain.Option{Value: o.Value, Label: o.Label}
	}

	return out
}

// toUsecaseAnswers adapts PlanFixture.Answers to the shape
// internal/adapter/planner/stub's AnswersKey takes. See toSpecSourceServices.
func toUsecaseAnswers(answers []Answer) []usecase.Answer {
	out := make([]usecase.Answer, 0, len(answers))
	for _, a := range answers {
		out = append(out, usecase.Answer{Param: a.Param, Value: a.Value})
	}

	return out
}

// newWorkspaceHandler wraps db (already open on cfg.DBPath - see build) in
// the workspaces usecase and its handler.
//
// permissions is threaded in from build, which opens it before this call:
// Workspaces.AddPanel narrows the catalogue by the caller's own
// permissions the same way Orchestrator.Invoke does
// (docs/specs/dashboard.md, AC-P-107), and needs a PermissionStore to do
// it.
//
// db, once opened, is never closed: it lives for the process's lifetime,
// same as the catalogue and the invoker's HTTP client above, with nothing
// in this package's own lifecycle to close it from.
func newWorkspaceHandler(db *sql.DB, catalog domain.Catalog, permissions usecase.PermissionStore) *handler.Workspace {
	store := sqlitestore.NewFromDB(db)

	return handler.NewWorkspace(usecase.NewWorkspaces(store, catalog, permissions))
}

// newPermissionStore wraps db (already open on cfg.DBPath - see build) as
// a usecase.PermissionStore, the same file newWorkspaceHandler and newAuth
// wrap their own tables of.
func newPermissionStore(db *sql.DB) usecase.PermissionStore {
	return sqlitestore.NewPermissionsFromDB(db)
}

// newAuth builds what Task 2's session endpoints, and Task 3's admin ones,
// need - a usecase.Authenticator, a usecase.SessionStore and the
// *sqlitestore.Users store itself (which also satisfies usecase.UserStore,
// for GET /api/users - handler.NewUsers's own Admin usecase, wired in
// build), all wrapping db (already open on cfg.DBPath - see build).
// sessions is always a real, non-nil store by the time it reaches
// httpserver.NewRouter - there is no longer a nil case for requireSession
// to answer by running every request as a fixed admin (see that function's
// doc comment for why that bypass was removed entirely, not just made
// harder to reach).
//
// Wrapping the accounts table also seeds the first admin from
// adminPassword when none exists yet (docs/specs/auth.md, section 3;
// docs/plans/auth.md, Task 0, Step 3) - so a bad ORCHESTRA_ADMIN_PASSWORD
// fails startup here, the same reason newPlanner's catalogue fetch does,
// above.
func newAuth(db *sql.DB, adminPassword string) (usecase.Authenticator, usecase.SessionStore, *sqlitestore.Users, error) {
	users := sqlitestore.NewUsersFromDB(db)

	authenticator, err := local.New(context.Background(), users, adminUserName, adminPassword)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("seeding the admin account: %w", err)
	}

	sessions := sqlitestore.NewSessionsFromDB(db)

	return authenticator, sessions, users, nil
}

// seedAccounts puts every SeedAccount in accounts in place - creating each
// one, by name, when it does not already exist (local.EnsureAccount) -
// once the admin account is already seeded. Not reachable through any
// HTTP route: cmd/api only ever populates Config.SeedAccounts from
// ORCHESTRA_SEED_ACCOUNTS, an environment variable read once before the
// process serves its first request, never set in production
// (internal/infra/config.Config.SeedAccounts) - the platform's
// composition root, the same seam AdminPassword itself uses, not a
// network-reachable "anyone can register" endpoint (docs/specs/auth.md,
// section 8: no account creation through the UI or the API).
func seedAccounts(ctx context.Context, users *sqlitestore.Users, accounts []SeedAccount) error {
	for _, account := range accounts {
		if _, err := local.EnsureAccount(ctx, users, account.Name, account.Password, domain.Role(account.Role)); err != nil {
			return fmt.Errorf("seeding account %q: %w", account.Name, err)
		}
	}

	return nil
}

// contextWindowOption turns Config.ContextTurns into the usecase.Option
// NewOrchestrator is built with. Zero - Config's own zero value, which
// every test that does not care about the window leaves unset - is left
// alone rather than passed through to usecase.WithContextWindow, which
// would collapse the window to zero turns kept; that keeps
// usecase.DefaultContextWindow in force instead, the same default the
// stub-only tests already relied on before turns existed.
func contextWindowOption(turns int) usecase.Option {
	if turns <= 0 {
		return func(*usecase.Orchestrator) {}
	}

	return usecase.WithContextWindow(turns)
}

// newNarrowerOption builds the usecase.Option newPlanner's own caller
// (build) passes to usecase.NewOrchestrator: a no-op when cfg.Narrowing is
// unconfigured (H7 - the zero value, which is what every test and every
// caller that predates this subproject leaves it at), or
// usecase.WithNarrower over an internal/adapter/narrowing/llamaswap.Narrower
// whose Load has already run against catalog, once, before this ever
// returns (AC-H-104: "a request embeds only the question").
//
// One line is logged with the vector count and how long Load took
// (docs/plans/shortlisting.md, Task 1 Step 6) - the same evidence H4's
// "the measurement asserts that no request paid for a load" is read from:
// a load that took tens of seconds here, at startup, is fine; the same
// delay inside a request would not be.
func newNarrowerOption(cfg *Config, catalog domain.Catalog) (usecase.Option, error) {
	if cfg.Narrowing.EmbedModel == "" {
		return func(*usecase.Orchestrator) {}, nil
	}

	narrower := llamaswap.New(cfg.LLM.BaseURL, cfg.Narrowing.EmbedModel, cfg.Narrowing.RerankModel)

	ctx := context.Background()
	start := time.Now()

	if err := narrower.Load(ctx, catalog); err != nil {
		return nil, fmt.Errorf("loading the narrowing index: %w", err)
	}

	slog.Default().InfoContext(ctx, "narrowing loaded",
		slog.Int("vectors", narrower.VectorCount()), slog.Duration("took", time.Since(start)))

	return usecase.WithNarrower(narrower, cfg.Narrowing.K), nil
}

// newPlanner selects the platform's usecase.Planner from cfg: the
// tool-calling adapter (internal/adapter/planner/toolcall) when an LLM
// base URL is configured, the stub over PlanFixtures otherwise.
//
// The stub is not just a test double here - it is production's fallback
// whenever no LLM is configured, which is why `make check`'s tests that
// build a Config with no LLM field set (every one of them; see
// app_test.go) never call a real one: they simply exercise this same
// fallback path.
func newPlanner(cfg *Config, catalog domain.Catalog) (usecase.Planner, error) {
	if cfg.LLM.BaseURL == "" {
		return stubplanner.New(toStubTable(cfg.PlanFixtures), &usecase.Decision{Kind: usecase.DecisionNone}), nil
	}

	client := chat.New(chat.Config{BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model})

	switch cfg.LLM.Mode {
	case "", ModeToolCall:
		w, ok := resolveWording(cfg.LLM.Wording)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrInvalidPlannerWording, cfg.LLM.Wording)
		}

		return toolcall.New(client, catalog, toolcallOptions(cfg, &w)...), nil
	case ModeJSON:
		return jsonmode.New(client, catalog, jsonmodeOptions(cfg)...), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidLLMMode, cfg.LLM.Mode)
	}
}

// toolcallOptions builds the toolcall.Option list newPlanner passes to
// toolcall.New: w always (WithWording), plus WithThinking when
// cfg.LLM.Thinking is set and WithRepeatPenalty when cfg.LLM.RepeatPenalty
// is set - both left off entirely when unset, so New's own defaults
// (thinking on, no repeat penalty) apply exactly as they did before either
// option existed.
func toolcallOptions(cfg *Config, w *wording.Wording) []toolcall.Option {
	opts := []toolcall.Option{toolcall.WithWording(w)}

	if cfg.LLM.Thinking != nil {
		opts = append(opts, toolcall.WithThinking(*cfg.LLM.Thinking))
	}

	if cfg.LLM.RepeatPenalty != nil {
		opts = append(opts, toolcall.WithRepeatPenalty(*cfg.LLM.RepeatPenalty, cfg.LLM.RepeatLastN))
	}

	if cfg.LLM.Today != nil {
		opts = append(opts, toolcall.WithClock(fixedClock(*cfg.LLM.Today)))
	}

	return opts
}

// jsonmodeOptions builds the jsonmode.Option list newPlanner passes to
// jsonmode.New: WithClock when cfg.LLM.Today is set, left off entirely
// otherwise so New's own default (time.Now) applies exactly as it did
// before this option existed - the same reasoning toolcallOptions already
// applies to toolcall.WithClock.
func jsonmodeOptions(cfg *Config) []jsonmode.Option {
	if cfg.LLM.Today == nil {
		return nil
	}

	return []jsonmode.Option{jsonmode.WithClock(fixedClock(*cfg.LLM.Today))}
}

// fixedClock returns a clock func that always answers today, regardless
// of when it is called - what toolcall.WithClock/jsonmode.WithClock need
// from Config.LLM.Today.
func fixedClock(today time.Time) func() time.Time {
	return func() time.Time { return today }
}

// resolveWording looks name up via wording.ByName, treating "" as
// wording.Default() - the same "empty means the default, anything else
// must be known" shape newPlanner's own Mode switch already has.
func resolveWording(name string) (wording.Wording, bool) {
	if name == "" {
		return wording.Default(), true
	}

	return wording.ByName(name)
}
