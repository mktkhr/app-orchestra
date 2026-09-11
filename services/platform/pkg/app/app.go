// Package app is the platform's composition root: the only public package,
// and the only place that wires the object graph together.
package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	invokerhttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/invoker/http"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	stubplanner "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/stub"
	specsourcehttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/specsource/http"
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
// produce. The stub is the only Planner that exists until Task 10 adds a
// real one, so this is also how the running platform answers a question
// today - see defaultPlanFixtures, which New falls back to when
// PlanFixtures is empty.
//
// Ask, when true, builds a DecisionAsk carrying Question and Param instead
// of a call; Service, OperationID and Args are unused for it. The
// candidate options themselves are never set here - Orchestrator.Plan
// looks them up from the catalogue (docs/plans/orchestration.md, Task 9),
// not from the fixture - so a fixture cannot hand out an option the
// catalogue would not.
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

// Config is everything New needs to wire the platform's object graph.
type Config struct {
	// StaticDir, when non-empty, is served at "/" as the built frontend.
	StaticDir string
	// Services is the catalogue's source: the platform fetches each
	// one's /openapi.yaml at startup and calls its operations at request
	// time. No service configured means an empty catalogue.
	Services []Service
	// PlanFixtures configures the stub planner. Empty - production's
	// default - falls back to defaultPlanFixtures, a small demo table so
	// the platform answers something without ever calling a real LLM
	// (make check's global constraint, docs/plans/orchestration.md).
	PlanFixtures []PlanFixture
}

// New builds the platform's http.Handler. Acceptance tests and cmd/api
// both go through this one function, so both exercise the same object
// graph; neither needs to see internal/, which is the point of pkg/app
// being the only public package.
func New(cfg Config) (http.Handler, error) {
	return build(cfg, httpserver.NewRouter)
}

// build takes the router constructor as a parameter so that the failure
// path - which httpserver.NewRouter cannot exercise with a valid, embedded
// spec - stays under test here.
func build(
	cfg Config,
	newRouter func(openapi.StrictServerInterface, string) (http.Handler, error),
) (http.Handler, error) {
	catalog, err := specsourcehttp.New(toSpecSourceServices(cfg.Services), nil).Fetch(context.Background())
	if err != nil {
		return nil, fmt.Errorf("building the catalogue: %w", err)
	}

	planner := stubplanner.New(toStubTable(planFixtures(cfg.PlanFixtures)), &usecase.Decision{Kind: usecase.DecisionNone})
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
		return usecase.Decision{Kind: usecase.DecisionAsk, Question: f.Question, Param: f.Param}
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

// planFixtures returns configured unchanged when it names at least one
// fixture, and defaultPlanFixtures() otherwise.
func planFixtures(configured []PlanFixture) []PlanFixture {
	if len(configured) > 0 {
		return configured
	}

	return defaultPlanFixtures()
}

// serviceInventory and paramStatus name the inventory service and its
// "status" parameter, each repeated across several fixtures below
// (goconst, part of the fixed harness policy, wants a repeated literal
// named once).
const (
	serviceInventory = "inventory"
	paramStatus      = "status"
)

// defaultPlanFixtures is the demo table the running platform answers with
// when Config.PlanFixtures is empty (production's case, until Task 10
// replaces the stub with a real planner): one fixed Japanese question maps
// to one safe list call per dummy service, so a person can see a rendered
// result end to end without hand-wiring anything. See
// docs/plans/orchestration.md Task 6 and STATE.md for how this is verified
// against the services running on localhost.
func defaultPlanFixtures() []PlanFixture {
	return []PlanFixture{
		{
			Query:       "在庫の一覧を見せて",
			Service:     serviceInventory,
			OperationID: "ListInventoryItems",
		},
		{
			Query:       "勤怠記録の一覧を見せて",
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
		},
		{
			Query:       "在庫を登録して",
			Service:     serviceInventory,
			OperationID: "CreateInventoryItem",
			Args:        map[string]any{"name": "デモ棚卸資産", paramStatus: "allocated", "quantity": 1},
		},
		{
			// "破損" (damaged) names no ItemStatus value: a disambiguation
			// question comes back naming the "status" parameter, with its
			// options built from the inventory catalogue's own enum (D11,
			// docs/specs/orchestration.md), not listed here.
			Query:    "破損した在庫を見せて",
			Ask:      true,
			Question: "「破損」に近いステータスはどれですか？",
			Param:    paramStatus,
		},
		{
			Query:       "破損した在庫を見せて",
			Answers:     []Answer{{Param: paramStatus, Value: "quarantined"}},
			Service:     serviceInventory,
			OperationID: "ListInventoryItems",
			Args:        map[string]any{paramStatus: "quarantined"},
		},
	}
}
