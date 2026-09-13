package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// DecisionKind names what a Planner decided to do about a question: call
// one endpoint, ask the user to disambiguate a value, list what the
// catalogue can do, or report that nothing in the catalogue fits.
type DecisionKind string

// The five things a Planner can decide (docs/specs/orchestration.md,
// section 4; docs/specs/proposing.md, section 3-4). All five are acted on
// by Orchestrator.
const (
	DecisionCall             DecisionKind = "call"
	DecisionAsk              DecisionKind = "ask"
	DecisionNone             DecisionKind = "none"
	DecisionListCapabilities DecisionKind = "list_capabilities"
	// DecisionProposal is what a propose_panel tool call (or its jsonmode
	// equivalent) maps onto: the model answering with a panel it composed
	// rather than doing anything (N1, docs/specs/proposing.md) - it is
	// never invoked, never written, and Orchestrator.propose never
	// touches the invoker over it.
	DecisionProposal DecisionKind = "propose_panel"
)

// Answer is the user's answer to a previous ask_user question, resubmitted
// alongside the original query so the planner can try again with it.
type Answer struct {
	Param string
	Value string
}

// Decision is what a Planner returns: which endpoint to call and with what
// arguments, or a question to ask, or nothing.
type Decision struct {
	Kind DecisionKind

	// Populated when Kind is DecisionCall or DecisionAsk. For DecisionAsk,
	// this is the operation the model was stuck on - the one it would have
	// called instead of ask_user, had the parameter's value been clear -
	// and it is what optionsForParam (orchestrator.go) uses to find the
	// one endpoint decision.Param belongs to: a parameter name such as
	// "status" or "type" is not unique across a catalogue of many
	// services, so the operation must be named alongside it.
	//
	// For DecisionListCapabilities, Service is reused for a different
	// purpose: list_capabilities' own optional "service" argument, which
	// narrows the catalogue listing to one service instead of naming the
	// service an operation belongs to. Empty means "every service".
	// OperationID is not used for this kind.
	Service     string
	OperationID string
	Args        map[string]any

	// Populated when Kind is DecisionAsk.
	Question string
	Param    string
	Options  []domain.Option

	// Populated when Kind is DecisionProposal, from propose_panel's own
	// optional arguments (docs/specs/proposing.md, section 3). Each is
	// the model's own value when it gave one, and left zero/nil
	// otherwise - Orchestrator.propose is what fills a zero value in from
	// the catalogue, never the planner (section 4: "the platform fills
	// in what the model left out").
	Component domain.Component
	View      *domain.View
	Title     string
}

// Turn is one earlier question in the conversation, and what the platform
// decided for it (docs/specs/context.md, section 3): a service, an
// operation and the arguments the decision was made with. Kind uses
// ResultKind, not DecisionKind - a Turn records what /api/plan answered
// (result, form, ask or none), which is what the wire's Turn.kind names
// too, not what Planner.Plan itself returned for the call that produced
// it.
//
// Turn has no field for the answer's data. That is deliberate (M1,
// docs/specs/context.md, section 3): a Turn is built only from what a
// Planner or Orchestrator already had to hand back to the caller anyway,
// so there is nowhere a row of a result could be smuggled in later without
// changing this type first.
type Turn struct {
	Question    string
	Kind        ResultKind
	Service     string
	OperationID string
	Args        map[string]any
}

// Planner decides, from a question, the conversation before it and the
// tools describing the catalogue, what to do about it. turns is the
// window Orchestrator.Plan has already truncated to ORCHESTRA_CONTEXT_TURNS
// (docs/specs/context.md, section 6) - a Planner never sees more of the
// conversation than that. Implemented by internal/adapter/planner/stub (a
// deterministic table lookup, used by every test and, by default, by the
// running platform - see pkg/app), which ignores turns (a table lookup has
// no prompt to render them into), and by the toolcall and jsonmode
// adapters that call a real LLM, which both render turns into the prompt
// after the catalogue - the tool definitions and the rendered catalogue
// text, respectively (M3, docs/specs/context.md section 4;
// docs/plans/context.md Task 2).
type Planner interface {
	Plan(ctx context.Context, query string, answers []Answer, turns []Turn, tools []Tool) (Decision, error)
}

// Invoker calls one endpoint of one service and returns its decoded JSON
// result. Implemented by internal/adapter/invoker/http.
//
// e is a pointer, not the value shown in the plan, because domain.Endpoint
// is 136 bytes: golangci-lint's gocritic hugeParam check (part of the fixed
// harness policy, see harness/quality/go/golangci.yml) rejects passing it
// by value - the same reason domain.Render and Endpoint.IsSafe take one.
type Invoker interface {
	Invoke(ctx context.Context, e *domain.Endpoint, args map[string]any) (any, error)
}
