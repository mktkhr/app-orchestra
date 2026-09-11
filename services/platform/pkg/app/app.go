// Package app is the platform's composition root: the only public package,
// and the only place that wires the object graph together.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	invokerhttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/invoker/http"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jsonmode"
	stubplanner "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/stub"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
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

// PlanFixture is one entry of the stub planner's table
// (internal/adapter/planner/stub): a question - together with the answers,
// if any, it was resubmitted with - mapped to the Decision it should
// produce. Used only when Config.LLM.BaseURL is empty (see newPlanner) - the
// running platform answers through the real, tool-calling planner
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
type PlanFixture struct {
	Query   string
	Answers []Answer

	Ask      bool
	Question string
	Param    string

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
	// DBPath is the SQLite file workspaces are kept in
	// (docs/specs/workspaces.md, W3). Empty skips building the workspace
	// store entirely, which is what every test in this package that
	// predates workspaces does; cmd/api always sets it, because
	// internal/infra/config.Load refuses to start without
	// ORCHESTRA_DB_PATH (config.ErrMissingDBPath) - the empty case here
	// exists for this package's own tests, not for production silently
	// doing without storage.
	DBPath string
}

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
	newRouter func(openapi.StrictServerInterface, string) (http.Handler, error),
) (http.Handler, error) {
	catalog, err := specsourcehttp.New(toSpecSourceServices(cfg.Services), nil).Fetch(context.Background())
	if err != nil {
		return nil, fmt.Errorf("building the catalogue: %w", err)
	}

	if dbErr := checkWorkspaceStore(cfg.DBPath); dbErr != nil {
		return nil, dbErr
	}

	planner, err := newPlanner(cfg, catalog)
	if err != nil {
		return nil, err
	}

	invoker := invokerhttp.New(toInvokerServices(cfg.Services), nil)
	orchestrator := usecase.NewOrchestrator(catalog, planner, invoker)

	api := handler.NewAPI(handler.NewHealth(), handler.NewPlan(orchestrator), handler.NewInvoke(orchestrator))

	router, err := newRouter(api, cfg.StaticDir)
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
// internal/adapter/planner/stub takes: each fixture's query and answers
// become its stub.Key, and Ask selects a DecisionAsk instead of the
// default DecisionCall.
func toStubTable(fixtures []PlanFixture) map[stubplanner.Key]usecase.Decision {
	table := make(map[stubplanner.Key]usecase.Decision, len(fixtures))

	for _, f := range fixtures {
		key := stubplanner.Key{Query: f.Query, Answers: stubplanner.AnswersKey(toUsecaseAnswers(f.Answers))}
		table[key] = toDecision(&f)
	}

	return table
}

// toDecision builds the Decision one PlanFixture produces: an ask when
// Ask is set, a call otherwise. See PlanFixture's doc comment for why an
// ask fixture carries no options of its own.
func toDecision(f *PlanFixture) usecase.Decision {
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

// toUsecaseAnswers adapts PlanFixture.Answers to the shape
// internal/adapter/planner/stub's AnswersKey takes. See toSpecSourceServices.
func toUsecaseAnswers(answers []Answer) []usecase.Answer {
	out := make([]usecase.Answer, 0, len(answers))
	for _, a := range answers {
		out = append(out, usecase.Answer{Param: a.Param, Value: a.Value})
	}

	return out
}

// checkWorkspaceStore opens and immediately closes the workspace store at
// dbPath, so a bad ORCHESTRA_DB_PATH (an unwritable directory, say) fails
// startup here rather than on the first request that touches it - the
// same reason newPlanner's catalogue fetch happens eagerly, above. An
// empty dbPath skips the check entirely: see Config.DBPath's doc comment
// for who leaves it empty and why that is fine.
//
// docs/plans/workspaces.md, Task 0 stops here - nothing yet keeps the
// store open or hands it to a handler, because nothing in this task
// speaks HTTP. Task 1 replaces this open-then-close with an open-and-keep
// once a handler exists to give it to.
func checkWorkspaceStore(dbPath string) error {
	if dbPath == "" {
		return nil
	}

	store, err := sqlitestore.New(dbPath)
	if err != nil {
		return fmt.Errorf("opening the workspace store: %w", err)
	}

	if err := store.Close(); err != nil {
		return fmt.Errorf("closing the workspace store: %w", err)
	}

	return nil
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
		return toolcall.New(client, catalog), nil
	case ModeJSON:
		return jsonmode.New(client, catalog), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidLLMMode, cfg.LLM.Mode)
	}
}
