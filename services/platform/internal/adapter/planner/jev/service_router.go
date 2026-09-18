// service_router.go: this package's own usecase.ServiceRouter (the
// full-catalogue Jev trial's own follow-up, docs/measurements/
// jev-full-catalogue.md; DECISIONS.md 2026-09-18) - a third call shape
// over the same POST /v1/systemone endpoint picker.go's Picker and
// gate.go's Gate already use, this time asking only which service a
// question belongs to: one "choice" question, one option per service the
// catalogue carries plus a catch-all ("other") for a question the
// catalogue does not cover at all
// (docs.typesafe.ai/primitives/choice recommends a catch-all whenever the
// option list may not cover the input). The package's own doc comment is
// client.go's.

package jev

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// routeQuestionName is the one key this file's own wireRequest.Questions
// and wireResponse.Answers ever use - this stage's own counterpart to
// picker.go's questionName ("pick") and gate.go's gateQuestionName
// ("gate").
const routeQuestionName = "route"

// routeOtherID is the catch-all option every service router request
// carries alongside one option per service the catalogue holds: "none of
// these services, or the question is not about any of them" - what wins
// when a question is out of scope for the whole catalogue, not merely
// hard to route. Winning this option is what mapRouteAnswer turns into
// ServiceRoute{}'s own "no opinion" - the same value an unrecognised
// choice or a router error both leave Plan with (fail open,
// internal/usecase/orchestrator.go's routeService).
const routeOtherID = "other"

// routeOtherPhraseV1 and routeOtherWhatV2/routeOtherExamplesV2 are
// routeOtherID's own CriteriaV1/CriteriaV2 wording - deliberately
// distinct from pick.PhraseNone (which means "no candidate operation
// answers this", a judgment about individual operations this stage never
// makes): here it means only "not about any of the services listed
// above".
const routeOtherPhraseV1 = "上記のどのサービスにも当てはまらない、またはサービスとは無関係な質問"

func routeOtherExamplesV2() []string {
	return []string{"今日の天気は？", "好きな食べ物は何？"}
}

// routeInstructions is the one question every service router request
// asks: which service (or the catch-all) query belongs to. Built to name
// no operation, and no built-in beyond the catch-all itself - this stage
// only ever narrows a shortlist to one service, never answers a question
// on its own (internal/usecase/service_router.go's own doc comment).
const routeInstructions = "この質問はどのサービスに関するものか。列挙されたサービスの説明を参考に1つ選ぶ。" +
	"どれにも当てはまらない、またはサービスと無関係な質問はotherを選ぶ。"

// deleteBuiltinCriteria removes the three fixed built-ins
// serviceCriteriaV1 always appends (pick.IDListCapabilities, IDProposePanel,
// IDNone) - this stage's own counterpart to hierarchical.go's
// opCriteriaFor, which does the same for one service's own "op_<service>"
// question. serviceCriteriaV1 is called with offerProposePanel false
// (routeCriteriaV1), so IDProposePanel is never actually present, but the
// delete is unconditional anyway - a no-op on an absent key costs
// nothing and keeps this helper correct if that ever changes.
func deleteBuiltinCriteria(criteria map[string]string) {
	delete(criteria, pick.IDListCapabilities)
	delete(criteria, pick.IDProposePanel)
	delete(criteria, pick.IDNone)
}

// deleteBuiltinCriteriaV2 is deleteBuiltinCriteria's CriteriaV2
// counterpart.
func deleteBuiltinCriteriaV2(criteria map[string]criterionV2) {
	delete(criteria, pick.IDListCapabilities)
	delete(criteria, pick.IDProposePanel)
	delete(criteria, pick.IDNone)
}

// routeCriteriaV1 builds the "route" question's own CriteriaV1 criteria:
// serviceCriteriaV1's own per-service lines (hierarchical.go), minus the
// three built-ins that function appends for the flat/hierarchical pick
// requests (this stage never offers list_capabilities, propose_panel or
// none - see routeInstructions' own doc comment), plus routeOtherID.
func routeCriteriaV1(catalog domain.Catalog, services []string) map[string]string {
	criteria := serviceCriteriaV1(catalog, services, false)

	deleteBuiltinCriteria(criteria)

	criteria[routeOtherID] = routeOtherPhraseV1

	return criteria
}

// routeCriteriaV2 is routeCriteriaV1's CriteriaV2 counterpart, built the
// same way over serviceCriteriaV2.
func routeCriteriaV2(catalog domain.Catalog, services []string) map[string]criterionV2 {
	criteria := serviceCriteriaV2(catalog, services, false)

	deleteBuiltinCriteriaV2(criteria)

	criteria[routeOtherID] = criterionV2{What: routeOtherPhraseV1, Examples: routeOtherExamplesV2()}

	return criteria
}

// buildServiceRouterRequest builds the one wireRequest ServiceRouter.Route
// sends for query, answers, turns and catalog: state/instructions handling
// shared with buildRequest/buildHierarchicalRequest (stateValue,
// routeInstructions fixed rather than switching on hasTurns/offerProposePanel
// - this stage never offers propose_panel and asks the same one question
// regardless of turns), one "route" question over routeCriteriaV1/V2.
func buildServiceRouterRequest(
	query string, answers []usecase.Answer, turns []usecase.Turn, catalog domain.Catalog, criteria string,
) wireRequest {
	services := servicesOf(catalog)

	var wireCriteria any = routeCriteriaV1(catalog, services)
	if criteria == CriteriaV2 {
		wireCriteria = routeCriteriaV2(catalog, services)
	}

	return wireRequest{
		State: stateValue(query, answers, turns, catalog),
		Model: modelName,
		Questions: map[string]wireQuestion{
			routeQuestionName: {Type: choiceQuestionType, Instructions: routeInstructions, Criteria: wireCriteria},
		},
	}
}

// mapRouteAnswer turns Jev's "route" answer into a usecase.ServiceRoute:
// routeOtherID (the catch-all) maps to ServiceRoute{} - "no opinion",
// fails open exactly as an unrecognised choice does - anything else is
// reported as-is, Service and Confidence carried straight through.
// Whether that Service is one the catalogue actually has is left to the
// caller (internal/usecase/orchestrator.go's routeService,
// catalogHasService): this function only decodes the wire answer, it
// does not validate it against catalog.
func mapRouteAnswer(answer wireAnswer) usecase.ServiceRoute {
	if answer.Choice == routeOtherID {
		return usecase.ServiceRoute{}
	}

	return usecase.ServiceRoute{Service: answer.Choice, Confidence: answer.Confidence}
}

// RouterOption configures a ServiceRouter built by NewServiceRouter.
type RouterOption func(*ServiceRouter)

// WithRouterCriteria selects which of routeCriteriaV1 (CriteriaV1) or
// routeCriteriaV2 (CriteriaV2) buildServiceRouterRequest uses - the same
// ORCHESTRA_JEV_CRITERIA switch Picker's own WithCriteria reads, so a
// deployment's router and picker always speak the same criteria wording
// (this file's own doc comment: "so the wording matches what was
// measured"). "" (WithRouterCriteria never given) is CriteriaV1, matching
// WithCriteria's own fallback.
func WithRouterCriteria(criteria string) RouterOption {
	return func(r *ServiceRouter) {
		if criteria == CriteriaV2 {
			r.criteria = CriteriaV2

			return
		}

		r.criteria = CriteriaV1
	}
}

// ServiceRouter implements usecase.ServiceRouter over TypeSafe's Jev API:
// one "choice" question naming every service the catalogue it is given
// carries, plus routeOtherID - see this file's own doc comment.
type ServiceRouter struct {
	client   *client
	criteria string
}

var _ usecase.ServiceRouter = (*ServiceRouter)(nil)

// NewServiceRouter builds a ServiceRouter calling baseURL with apiKey. A
// nil httpClient defaults to http.DefaultClient - same as New and
// NewGate; this adapter never shares a *client instance with either of
// them (see NewGate's own doc comment for why).
func NewServiceRouter(baseURL, apiKey string, httpClient *http.Client, opts ...RouterOption) *ServiceRouter {
	r := &ServiceRouter{client: newClient(baseURL, apiKey, httpClient)}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Route sends query, answers, turns and catalog to Jev as one "route"
// choice question (buildServiceRouterRequest) and reports the winning
// service (mapRouteAnswer). An empty catalog has no service to route to
// at all - it returns usecase.ServiceRoute{} without calling the API,
// same short-circuit Picker.Pick and Gate.Gate take for an empty
// shortlist. The call is bounded to requestTimeout regardless of ctx's
// own deadline, same as Pick/Gate.
//
// Errors are returned, not swallowed: internal/usecase/orchestrator.go's
// routeService is where a router outage fails open, by design - this
// method's job is only to report what happened, truthfully.
func (r *ServiceRouter) Route(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, catalog domain.Catalog,
) (usecase.ServiceRoute, error) {
	if len(catalog.Endpoints) == 0 {
		return usecase.ServiceRoute{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	start := time.Now()

	req := buildServiceRouterRequest(query, answers, turns, catalog, r.criteria)

	resp, err := r.client.pick(ctx, req)
	if err != nil {
		return usecase.ServiceRoute{}, fmt.Errorf("jev service routing: %w", err)
	}

	answer, ok := resp.Answers[routeQuestionName]
	if !ok {
		return usecase.ServiceRoute{}, fmt.Errorf("%w: response named no %q answer", ErrRequestFailed, routeQuestionName)
	}

	route := mapRouteAnswer(answer)

	slog.Default().InfoContext(ctx, "service route completed",
		slog.String("route_choice", answer.Choice),
		slog.Float64("route_confidence", answer.Confidence),
		slog.Int64("route_ms", time.Since(start).Milliseconds()),
		slog.Int("route_input_tokens", resp.Usage.InputTokens),
		slog.Int("route_output_tokens", resp.Usage.OutputTokens),
		// route_probabilities is Jev's own full per-service probability
		// distribution for this "route" answer, logged the same reason
		// Picker.Pick's own pick_probabilities is (mapping.go): so a
		// run's platform log alone is enough to re-analyse a whole run's
		// worth of routing decisions without a second, debug-only line.
		slog.Any("route_probabilities", answer.Probabilities))

	return route, nil
}
