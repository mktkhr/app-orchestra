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
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
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
// is not unique across services. The candidate options themselves are
// never set here - Orchestrator.Plan looks them up from that endpoint
// (docs/plans/orchestration.md, Task 9), not from the fixture - so a
// fixture cannot hand out an option the catalogue would not.
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
	orchestratorOptions := append(
		[]usecase.Option{contextWindowOption(cfg.ContextTurns), narrowerOption}, stagingOptions(cfg)...,
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
// doc comment). See PlanFixture's doc comment for why an ask fixture
// carries no options of its own.
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
		return jsonmode.New(client, catalog), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidLLMMode, cfg.LLM.Mode)
	}
}

// stagesTwo is the one value of Config.LLM.Stages that turns staging on
// (docs/specs/staging.md, S6) - named so the comparison below isn't a bare
// magic number (mnd, harness/quality/go/golangci.yml).
const stagesTwo = 2

// stagingOptions builds the usecase.Option list build passes to
// NewOrchestrator for the staging subproject: nil when cfg.LLM.Stages is
// not 2, or when cfg.LLM.BaseURL is empty (every test and caller that
// predates this subproject, and production with no LLM configured -
// newPlanner's own stub fallback, above), otherwise a pick.Picker built
// over this same LLM (S6: "the pick's model is the planner's model, its
// base URL the planner's") plus usecase.WithStages(2). Staging needs a
// real model to pick against; pairing it with the stub planner would send
// a real request to an empty base URL instead of exercising the stub.
//
// The picker gets its own chat.Client rather than sharing newPlanner's -
// newPlanner returns only a usecase.Planner, not the client it built, and
// a second *http.Client here costs nothing a request-scoped call would
// notice.
func stagingOptions(cfg *Config) []usecase.Option {
	if cfg.LLM.Stages != stagesTwo || cfg.LLM.BaseURL == "" {
		return nil
	}

	client := chat.New(chat.Config{BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model})
	picker := pick.New(client, cfg.LLM.Model)

	return []usecase.Option{usecase.WithPicker(picker), usecase.WithStages(stagesTwo)}
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

	return opts
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
