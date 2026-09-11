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

// PlanFixture is one entry of the stub planner's table
// (internal/adapter/planner/stub): an exact question mapped to the call it
// should produce. The stub is the only Planner that exists until Task 10
// adds a real one, so this is also how the running platform answers a
// question today - see defaultPlanFixtures, which New falls back to when
// PlanFixtures is empty.
type PlanFixture struct {
	Query       string
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
// internal/adapter/planner/stub takes: an exact query maps to a
// DecisionCall against the named operation.
func toStubTable(fixtures []PlanFixture) map[string]usecase.Decision {
	table := make(map[string]usecase.Decision, len(fixtures))

	for _, f := range fixtures {
		table[f.Query] = usecase.Decision{
			Kind:        usecase.DecisionCall,
			Service:     f.Service,
			OperationID: f.OperationID,
			Args:        f.Args,
		}
	}

	return table
}

// planFixtures returns configured unchanged when it names at least one
// fixture, and defaultPlanFixtures() otherwise.
func planFixtures(configured []PlanFixture) []PlanFixture {
	if len(configured) > 0 {
		return configured
	}

	return defaultPlanFixtures()
}

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
			Service:     "inventory",
			OperationID: "ListInventoryItems",
		},
		{
			Query:       "勤怠記録の一覧を見せて",
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
		},
		{
			Query:       "在庫を登録して",
			Service:     "inventory",
			OperationID: "CreateInventoryItem",
			Args:        map[string]any{"name": "デモ棚卸資産", "status": "allocated", "quantity": 1},
		},
	}
}
