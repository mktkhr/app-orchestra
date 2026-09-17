// hierarchical.go: this package's own answer to a shortlist that exceeds
// Jev's own 255-option "choice" limit (docs.typesafe.ai/primitives/choice)
// - the full-catalogue Jev trial (narrowing off, up to 1000 operations
// across 5 services) that motivated this file. Rather than truncate the
// shortlist, buildHierarchicalRequest follows
// docs.typesafe.ai/cookbooks/hierarchical_classification.md: one "service"
// choice question (every service present in the shortlist, plus the same
// three built-ins buildRequest's own flat "pick" question carries), then
// one "op_<service>" choice question per service (that service's own
// endpoints only, no built-ins). All of it is still one HTTP call - Jev
// evaluates every question of one request in parallel (the same "fan-out,
// not extra calls" property WithFanOutGate already relies on,
// docs/measurements/jev-picker-v5.md) - so this is not the two-call
// alternative (a Gate-style pre-flight, then pick) the same docs page also
// shows.
//
// Picker.Pick (picker.go) decides which shape to send: buildRequest
// unchanged when shortlist's own endpoint count plus the three built-ins
// fits within 255 (needsHierarchical false - the byte-identical path every
// pre-existing test in picker_test.go covers), buildHierarchicalRequest
// otherwise.

package jev

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// maxChoiceOptions is Jev's own documented ceiling on one "choice"
// question's own option count (docs.typesafe.ai/primitives/choice: up to
// 255 options, hierarchical classification recommended above that) -
// needsHierarchical's own threshold.
const maxChoiceOptions = 255

// serviceQuestionName is the hierarchical request's own top-level
// question: which service (or built-in) the query belongs to.
const serviceQuestionName = "service"

// opQuestionPrefix names each hierarchical request's own per-service
// question, opQuestionPrefix+service - opQuestionName builds the full key.
const opQuestionPrefix = "op_"

// opQuestionName is one service's own "op_<service>" question key.
func opQuestionName(service string) string {
	return opQuestionPrefix + service
}

// ErrServiceTooLarge is returned by buildHierarchicalRequest when a single
// service's own shortlist endpoints alone exceed maxChoiceOptions - the
// "op_<service>" question would itself need hierarchical classification,
// which this package does not build a third level for (not expected
// against this trial's own fixture, whose largest service is 200
// operations).
var ErrServiceTooLarge = errors.New("jev hierarchical: service has more than 255 endpoints")

// needsHierarchical reports whether shortlist's own endpoint count, plus
// the three fixed built-ins buildRequest's flat "pick" question always
// carries, exceeds maxChoiceOptions - Picker.Pick's own switch between
// buildRequest (<=255, byte-identical to every request this adapter sent
// before this file existed) and buildHierarchicalRequest.
func needsHierarchical(shortlist domain.Catalog) bool {
	return len(shortlist.Endpoints)+builtinCriteriaCount > maxChoiceOptions
}

// servicesOf returns the distinct services shortlist's own endpoints
// belong to, in first-seen order - deterministic, since Pick always
// receives shortlist in the narrowed catalogue's own already-deterministic
// order. Both buildHierarchicalRequest (which services get their own "op_"
// question) and hierarchicalCandidates (which services get scored) call
// this, so the two never disagree on what "every service present in the
// shortlist" means.
func servicesOf(shortlist domain.Catalog) []string {
	seen := make(map[string]bool, len(shortlist.Endpoints))

	var services []string

	for i := range shortlist.Endpoints {
		s := shortlist.Endpoints[i].Service
		if seen[s] {
			continue
		}

		seen[s] = true

		services = append(services, s)
	}

	return services
}

// endpointsForService returns the subset of shortlist's own endpoints
// belonging to service, in shortlist's own order - one "op_<service>"
// question's own option set.
func endpointsForService(shortlist domain.Catalog, service string) domain.Catalog {
	var endpoints []domain.Endpoint

	for i := range shortlist.Endpoints {
		if shortlist.Endpoints[i].Service == service {
			endpoints = append(endpoints, shortlist.Endpoints[i])
		}
	}

	return domain.Catalog{Endpoints: endpoints}
}

// serviceDescOpCount is how many of a service's own operations
// serviceOpSummaries folds into that service's own criterion - three,
// enough to give Jev a sense of the service's own scope without writing a
// hand-authored description per service (this package has no such
// description to draw on; the catalogue only ever gives it endpoints).
const serviceDescOpCount = 3

// serviceOpSummaries returns up to serviceDescOpCount of service's own
// endpoints' summaryFor lines, in shortlist order - serviceCriteriaV1/V2's
// own deterministic stand-in for a hand-written service description: the
// display name plus a few of its own operations' own display names/
// summaries, per this task's own instruction, rather than anything
// requiring a new catalogue field.
func serviceOpSummaries(shortlist domain.Catalog, service string) []string {
	var lines []string

	for i := range shortlist.Endpoints {
		e := &shortlist.Endpoints[i]
		if e.Service != service {
			continue
		}

		lines = append(lines, summaryFor(e))

		if len(lines) == serviceDescOpCount {
			break
		}
	}

	return lines
}

// serviceDisplayNameOf returns service's own display name: the first
// shortlist endpoint belonging to it, read through
// Endpoint.ServiceDisplayNameOr (falling back to the raw service id) -
// every endpoint of one service shares the same ServiceDisplayName in
// practice, so the first one found already speaks for the whole service.
func serviceDisplayNameOf(shortlist domain.Catalog, service string) string {
	for i := range shortlist.Endpoints {
		e := &shortlist.Endpoints[i]
		if e.Service == service {
			return e.ServiceDisplayNameOr(service)
		}
	}

	return service
}

// serviceCriteriaV1 builds the "service" question's own CriteriaV1
// criteria: one "<serviceDisplayName> / <op1>、<op2>、<op3>" line per
// service in services (serviceOpSummaries, 、-joined - the same
// "<display> / <summary>" shape criteriaFor's own per-endpoint line uses,
// so a person reading both questions' criteria side by side sees the same
// vocabulary), plus the same fixed built-ins criteriaFor itself appends -
// propose_panel only when offerProposePanel is true (O3, the same switch
// buildRequest's own criteriaFor call already applies).
func serviceCriteriaV1(shortlist domain.Catalog, services []string, offerProposePanel bool) map[string]string {
	criteria := make(map[string]string, len(services)+builtinCriteriaCount)

	for _, svc := range services {
		criteria[svc] = serviceDisplayNameOf(shortlist, svc) + " / " + strings.Join(serviceOpSummaries(shortlist, svc), "、")
	}

	criteria[pick.IDListCapabilities] = pick.PhraseListCapabilities

	if offerProposePanel {
		criteria[pick.IDProposePanel] = pick.PhraseProposePanel
	}

	criteria[pick.IDNone] = pick.PhraseNone

	return criteria
}

// serviceCriteriaV2 is serviceCriteriaV1's CriteriaV2 counterpart: each
// service's own criterionV2 carries its display name as What and
// serviceOpSummaries as Examples - the object form's own "what a person
// might want from this service" slot, filled the same deterministic way as
// serviceCriteriaV1's joined line - plus the same fixed built-ins
// criteriaForV2 itself appends, propose_panel only when offerProposePanel
// is true (see serviceCriteriaV1's own doc comment).
func serviceCriteriaV2(shortlist domain.Catalog, services []string, offerProposePanel bool) map[string]criterionV2 {
	criteria := make(map[string]criterionV2, len(services)+builtinCriteriaCount)

	for _, svc := range services {
		criteria[svc] = criterionV2{What: serviceDisplayNameOf(shortlist, svc), Examples: serviceOpSummaries(shortlist, svc)}
	}

	criteria[pick.IDListCapabilities] = criterionV2{What: pick.PhraseListCapabilities, Examples: []string{"何ができるの？"}}

	if offerProposePanel {
		criteria[pick.IDProposePanel] = criterionV2{What: pick.PhraseProposePanel}
	}

	criteria[pick.IDNone] = criterionV2{What: pick.PhraseNone, Examples: noneExamplesV2()}

	return criteria
}

// opCriteriaFor builds one "op_<service>" question's own criteria: the
// same criteriaFor (CriteriaV1) or criteriaForV2 (CriteriaV2) buildRequest
// itself uses, called on endpoints - that service's own endpoints alone,
// not the full shortlist. Calling criteriaForV2 this way also answers what
// notForV2's own cross-service "not_for" collision field becomes within one
// service's own question: always "" here, since notForV2 only ever names a
// *different* service sharing the same noun, and every option in this
// question already belongs to the one service the "service" question above
// resolved - there is no cross-service collision left within it to warn
// about. criteriaFor/criteriaForV2 both also append the three fixed
// built-ins; op questions never carry built-ins (this file's own doc
// comment), so those three keys are deleted again right after building.
func opCriteriaFor(endpoints domain.Catalog, criteria string) any {
	// offerProposePanel is passed true here regardless of the request's own
	// O3 switch: the entry is deleted again immediately below either way,
	// so which value criteriaFor/criteriaForV2 build it with never reaches
	// the wire.
	if criteria == CriteriaV2 {
		c := criteriaForV2(endpoints, true)
		delete(c, pick.IDListCapabilities)
		delete(c, pick.IDProposePanel)
		delete(c, pick.IDNone)

		return c
	}

	c := criteriaFor(endpoints, true)
	delete(c, pick.IDListCapabilities)
	delete(c, pick.IDProposePanel)
	delete(c, pick.IDNone)

	return c
}

// buildHierarchicalRequest builds the one wireRequest Picker.Pick sends
// when needsHierarchical(shortlist) is true: state/instructions handling
// identical to buildRequest's own (stateValue, defaultInstructionsFor/
// instructionsFor - the same wireQuestionInstructions value is reused
// unchanged across the "service" question and every "op_<service>"
// question, exactly as buildRequest's own single "pick" question carries
// it once), one "service" question (serviceCriteriaV1/V2) and one
// "op_<service>" question per service present in shortlist (opCriteriaFor).
// Returns ErrServiceTooLarge if any one service's own endpoints alone
// exceed maxChoiceOptions. planCtx.WorkspaceID != "" is this request's own
// offerProposePanel (O3), applied to the "service" question's own criteria
// and to wireQuestionInstructions exactly as buildRequest applies it to the
// flat "pick" question - every "op_<service>" question never carries
// built-ins at all (opCriteriaFor), so it needs no switch of its own.
func buildHierarchicalRequest(
	query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog,
	criteria string, objectInstructions bool, planCtx usecase.PlanContext,
) (wireRequest, error) {
	offerProposePanel := planCtx.WorkspaceID != ""

	services := servicesOf(shortlist)

	var wireServiceCriteria any = serviceCriteriaV1(shortlist, services, offerProposePanel)
	if criteria == CriteriaV2 {
		wireServiceCriteria = serviceCriteriaV2(shortlist, services, offerProposePanel)
	}

	var wireQuestionInstructions any = defaultInstructionsFor(criteria, offerProposePanel)
	if objectInstructions {
		wireQuestionInstructions = instructionsFor(criteria, len(turns) > 0, offerProposePanel)
	}

	questions := map[string]wireQuestion{
		serviceQuestionName: {
			Type: choiceQuestionType, Instructions: wireQuestionInstructions, Criteria: wireServiceCriteria,
		},
	}

	for _, svc := range services {
		endpoints := endpointsForService(shortlist, svc)
		if len(endpoints.Endpoints) > maxChoiceOptions {
			return wireRequest{}, fmt.Errorf("%w: service %q has %d endpoints", ErrServiceTooLarge, svc, len(endpoints.Endpoints))
		}

		questions[opQuestionName(svc)] = wireQuestion{
			Type: choiceQuestionType, Instructions: wireQuestionInstructions, Criteria: opCriteriaFor(endpoints, criteria),
		}
	}

	return wireRequest{State: stateValue(query, answers, turns, shortlist), Model: modelName, Questions: questions}, nil
}

// hierarchicalCandidate is one scored leaf of the hierarchical answer: a
// shortlist endpoint (Service and ID both set, ID its OperationID) or a
// built-in (Service "", ID one of pick.IDListCapabilities et al.). Score is
// what mapHierarchicalChosen and the "pick completed" log line both read -
// P(builtin) for a built-in (one decision), or
// sqrt(P(service) * P(op|service)) for an endpoint (two decisions'
// geometric mean, this file's own doc comment).
type hierarchicalCandidate struct {
	ID          string
	Service     string
	ServiceProb float64
	OpProb      float64
	Score       float64
}

// hierarchicalCandidates scores every candidate a hierarchical response can
// name: the three built-ins off the "service" question's own probabilities
// map, then every option resp names probabilities for in each service's own
// "op_<service>" answer, combined with that service's own probability off
// the "service" answer (this file's own doc comment: score =
// sqrt(P(service) * P(op|service))). A service or op answer resp does not
// name at all (should not happen against a request this package itself
// built, but resp is an external response) simply contributes nothing -
// not an error, since the other services/built-ins may still be enough to
// answer from. nil when resp carries no "service" answer at all.
func hierarchicalCandidates(resp wireResponse, shortlist domain.Catalog) []hierarchicalCandidate {
	serviceAnswer, ok := resp.Answers[serviceQuestionName]
	if !ok {
		return nil
	}

	candidates := make([]hierarchicalCandidate, 0, len(shortlist.Endpoints)+builtinCriteriaCount)

	for _, id := range [...]string{pick.IDListCapabilities, pick.IDProposePanel, pick.IDNone} {
		p := serviceAnswer.Probabilities[id]
		candidates = append(candidates, hierarchicalCandidate{ID: id, ServiceProb: p, Score: p})
	}

	for _, svc := range servicesOf(shortlist) {
		opAnswer, ok := resp.Answers[opQuestionName(svc)]
		if !ok {
			continue
		}

		pSvc := serviceAnswer.Probabilities[svc]

		for opID, pOp := range opAnswer.Probabilities {
			candidates = append(candidates, hierarchicalCandidate{
				ID: opID, Service: svc, ServiceProb: pSvc, OpProb: pOp, Score: math.Sqrt(pSvc * pOp),
			})
		}
	}

	return candidates
}

// candidateKey orders two equal-Score candidates deterministically -
// sortHierarchicalCandidates' own tie-break, so a fake transport with a
// flat probabilities map (every candidate 0, e.g.) still gives a
// reproducible "top 3" log line and a reproducible chosen candidate across
// runs, rather than depending on Go's own randomized map iteration order.
func candidateKey(c hierarchicalCandidate) string {
	return c.Service + "/" + c.ID
}

// sortHierarchicalCandidates sorts candidates by Score descending, ties
// broken by candidateKey ascending - highest-scoring, most-deterministic
// first.
func sortHierarchicalCandidates(candidates []hierarchicalCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}

		return candidateKey(candidates[i]) < candidateKey(candidates[j])
	})
}

// mapHierarchicalChosen turns the winning hierarchicalCandidate into a
// usecase.Pick - the hierarchical path's own counterpart to mapAnswer.
// Ambiguous is chosen.Score below threshold: chosen.Score sits on the same
// [0, 1] scale mapAnswer's own answer.Confidence does (a single
// probability for a built-in; a geometric mean of two probabilities,
// itself a probability, for an endpoint), so the same ambiguityThreshold
// this Picker already uses for the flat path (mapAnswer, S4) is reused
// unchanged rather than inventing a second threshold - the closest mirror
// of the flat rule the hierarchical data supports, since there is no
// single Jev-judged "confidence" field for a two-decision path the way
// there is for the flat one-decision "pick" answer.
func mapHierarchicalChosen(chosen hierarchicalCandidate, threshold float64) usecase.Pick {
	ambiguous := chosen.Score < threshold

	switch chosen.ID {
	case pick.IDListCapabilities:
		return usecase.Pick{Kind: usecase.PickListCapabilities, Ambiguous: ambiguous, Confidence: chosen.Score}
	case pick.IDProposePanel:
		return usecase.Pick{Kind: usecase.PickProposePanel, Ambiguous: ambiguous, Confidence: chosen.Score}
	case pick.IDNone:
		return usecase.Pick{Kind: usecase.PickNone, Ambiguous: ambiguous, Confidence: chosen.Score}
	default:
		return usecase.Pick{
			Kind: usecase.PickOperation, Service: chosen.Service, OperationID: chosen.ID,
			Ambiguous: ambiguous, Confidence: chosen.Score,
		}
	}
}

// loggedPath is one hierarchicalCandidate as the "pick completed" log
// line's own pick_top3_paths carries it - a small, JSON-friendly subset
// (slog's JSON handler marshals this the same way it already marshals
// mapAnswer's own pick_probabilities map, picker_test.go's
// TestPickCompletedLogCarriesTheFullProbabilityDistribution).
type loggedPath struct {
	Service string  `json:"service"`
	ID      string  `json:"id"`
	Score   float64 `json:"score"`
}

// topPathCount is how many of hierarchicalCandidates' own sorted
// candidates pickHierarchical's own log line carries as pick_top3_paths -
// named so the call site isn't a bare magic number (mnd,
// harness/quality/go/golangci.yml).
const topPathCount = 3

// topPaths returns candidates' own top n scored paths (candidates must
// already be sorted, sortHierarchicalCandidates) as loggedPath, for
// Picker.pickHierarchical's own log line.
func topPaths(candidates []hierarchicalCandidate, n int) []loggedPath {
	if len(candidates) < n {
		n = len(candidates)
	}

	paths := make([]loggedPath, n)
	for i := range n {
		paths[i] = loggedPath{Service: candidates[i].Service, ID: candidates[i].ID, Score: candidates[i].Score}
	}

	return paths
}
