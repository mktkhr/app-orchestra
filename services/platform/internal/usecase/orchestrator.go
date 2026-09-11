package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// ResultKind mirrors the `kind` of the /api/plan HTTP contract
// (docs/specs/orchestration.md, section 6): result, form, ask or none.
type ResultKind string

// The four outcomes /api/plan can report. Only ResultKindResult and
// ResultKindNone are produced today; ResultKindForm (Task 7) and
// ResultKindAsk (Task 9) are named here so Result's shape does not change
// again once those tasks land.
const (
	ResultKindResult ResultKind = "result"
	ResultKindForm   ResultKind = "form"
	ResultKindAsk    ResultKind = "ask"
	ResultKindNone   ResultKind = "none"
)

// Result is what Orchestrator.Plan returns: the outcome of one question,
// shaped so a handler can render it directly onto the /api/plan response.
type Result struct {
	Kind ResultKind

	// Populated when Kind is ResultKindResult or ResultKindForm: which
	// endpoint the result came from, or the form would submit to.
	Service     string
	OperationID string

	// Populated when Kind is ResultKindResult.
	Component domain.Component
	Data      any
	Args      map[string]any

	// Populated when Kind is ResultKindNone.
	Message string

	// Populated when Kind is ResultKindForm: the request body's JSON
	// Schema, and the values the planner already filled in.
	Schema  map[string]any
	Initial map[string]any

	// Populated when Kind is ResultKindAsk (Task 9).
	Question string
	Param    string
	Options  []domain.Option
}

// ErrEndpointNotFound is returned when a Decision names an operation the
// catalogue does not have.
var ErrEndpointNotFound = errors.New("endpoint not found in catalogue")

// ErrNotImplemented marks a Decision this deployment does not act on yet: a
// disambiguation question, which needs the ask path of Task 9. Returned as
// a sentinel, rather than silently degrading to ResultKindNone, so a
// caller can distinguish "nothing fits" from "this isn't built yet".
var ErrNotImplemented = errors.New("not implemented")

// messageNoEndpoint is the message a ResultKindNone result carries: the
// planner itself decided nothing in the catalogue fits the question.
const messageNoEndpoint = "その質問に答えられる操作が見つかりませんでした。"

// formSchema builds the JSON Schema a form result carries: the endpoint's
// request body, converted with the same schemaToJSONSchema (tools.go) used
// to describe it to the planner, so the two never drift apart. An unsafe
// endpoint with no request body at all - not something the catalogue
// produces today, but not ruled out by the type system either - reports an
// empty object schema rather than dereferencing a nil Schema.
func formSchema(e *domain.Endpoint) map[string]any {
	if e.RequestBody == nil {
		return map[string]any{keyType: domain.SchemaTypeObject}
	}

	return schemaToJSONSchema(e.RequestBody)
}

// Orchestrator drives one /api/plan request: it asks Planner for a
// Decision over the catalogue's tools and, for a safe call, invokes it and
// renders the result (docs/specs/orchestration.md, section 4).
type Orchestrator struct {
	catalog domain.Catalog
	planner Planner
	invoker Invoker
}

// NewOrchestrator builds an Orchestrator over the given catalogue, planner
// and invoker.
func NewOrchestrator(catalog domain.Catalog, planner Planner, invoker Invoker) *Orchestrator {
	return &Orchestrator{catalog: catalog, planner: planner, invoker: invoker}
}

// Plan turns a question (plus any answers to a previous ask) into a
// Result. The safe-call, unsafe-call (form) and none paths are
// implemented; an ask decision returns an error wrapping ErrNotImplemented
// (Task 9 builds it).
func (o *Orchestrator) Plan(ctx context.Context, query string, answers []Answer) (Result, error) {
	tools := ToolsFor(o.catalog)

	decision, err := o.planner.Plan(ctx, query, answers, tools)
	if err != nil {
		return Result{}, fmt.Errorf("planning: %w", err)
	}

	switch decision.Kind {
	case DecisionNone:
		return Result{Kind: ResultKindNone, Message: messageNoEndpoint}, nil
	case DecisionCall:
		return o.call(ctx, &decision)
	case DecisionAsk:
		return Result{}, fmt.Errorf("%w: an ask_user decision (Task 9 builds this path)", ErrNotImplemented)
	default:
		return Result{}, fmt.Errorf("%w: unknown decision kind %q", ErrNotImplemented, decision.Kind)
	}
}

// call resolves a DecisionCall against the catalogue: a safe endpoint is
// invoked and rendered; an unsafe one (D8, docs/specs/orchestration.md)
// never reaches the service at all - it comes back as a form to confirm
// instead, carrying the request body's schema and the planner's arguments
// as initial values.
//
// decision is a pointer, not the value Plan holds, because Decision is 112
// bytes: golangci-lint's gocritic hugeParam check (part of the fixed
// harness policy, see harness/quality/go/golangci.yml) rejects passing it
// by value.
func (o *Orchestrator) call(ctx context.Context, decision *Decision) (Result, error) {
	endpoint, ok := o.catalog.Find(decision.Service, decision.OperationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, decision.Service, decision.OperationID)
	}

	if !endpoint.IsSafe() {
		return Result{
			Kind:        ResultKindForm,
			Service:     decision.Service,
			OperationID: decision.OperationID,
			Schema:      formSchema(&endpoint),
			Initial:     decision.Args,
		}, nil
	}

	data, err := o.invoker.Invoke(ctx, &endpoint, decision.Args)
	if err != nil {
		return Result{}, fmt.Errorf("invoking %s/%s: %w", decision.Service, decision.OperationID, err)
	}

	return Result{
		Kind:        ResultKindResult,
		Component:   domain.Render(&endpoint),
		Data:        data,
		Service:     decision.Service,
		OperationID: decision.OperationID,
		Args:        decision.Args,
	}, nil
}
