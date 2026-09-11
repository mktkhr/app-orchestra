package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"

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

// ErrInvalidArguments is returned when Invoke's arguments do not satisfy
// the endpoint's schema: a required request body field is missing, or a
// value is not one of an enum parameter's declared values.
var ErrInvalidArguments = errors.New("invalid arguments")

// ErrNotImplemented marks a Decision this deployment does not act on: a
// DecisionKind the switch in Plan does not recognise at all. Returned as a
// sentinel, rather than silently degrading to ResultKindNone, so a caller
// can distinguish "nothing fits" from "this isn't built yet".
var ErrNotImplemented = errors.New("not implemented")

// ErrUnknownParam is returned when a DecisionAsk names a parameter the
// catalogue does not declare as an enum anywhere. A Decision is ultimately
// produced by a model (docs/plans/orchestration.md Task 9), which can name
// a parameter that does not exist or is not an enum, so a disambiguation
// question is only ever answered from the catalogue's own declared
// values (see optionsForParam) - never by trusting Decision.Options,
// which the same model filled in and which may list values that do not
// exist.
var ErrUnknownParam = errors.New("parameter not found in catalogue")

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
// Result: a safe call is invoked and rendered, an unsafe call comes back as
// a form to confirm, an ask decision comes back as a disambiguation
// question built from the catalogue (see ask), and none reports that
// nothing fits.
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
		return o.ask(&decision)
	default:
		return Result{}, fmt.Errorf("%w: unknown decision kind %q", ErrNotImplemented, decision.Kind)
	}
}

// Invoke executes a confirmed call: it is what POST /api/invoke drives
// (docs/specs/orchestration.md, section 5). Unlike Plan, it never consults
// the planner - a person has already chosen the operation and its
// arguments by pressing a button, so there is nothing left to decide - and
// it acts on every method, safe or not: this is the one place an unsafe
// method actually runs, which is what keeps /api/plan's "the model cannot
// change anything on its own" property true (D8, docs/specs/orchestration.md
// section 2).
//
// TODO(auth): once authentication exists, the permission check for
// (service, operationID) belongs here, after the catalogue lookup and
// before validateArgs - this is the only mouth an unsafe method executes
// through, so it is the only place that check needs to be made.
func (o *Orchestrator) Invoke(ctx context.Context, service, operationID string, args map[string]any) (Result, error) {
	endpoint, ok := o.catalog.Find(service, operationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, service, operationID)
	}

	if err := validateArgs(&endpoint, args); err != nil {
		return Result{}, err
	}

	return o.invokeAndRender(ctx, &endpoint, service, operationID, args)
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

	return o.invokeAndRender(ctx, &endpoint, decision.Service, decision.OperationID, decision.Args)
}

// ask resolves a DecisionAsk into a ResultKindAsk: it never touches the
// invoker (an ask never calls a service), and it never repeats
// decision.Options verbatim - see optionsForParam for why the catalogue,
// not the model's own Decision, is the source of truth for what a person
// is offered to pick from.
//
// decision is a pointer for the same reason call's is: golangci-lint's
// gocritic hugeParam check on Decision's 112 bytes (see
// harness/quality/go/golangci.yml).
func (o *Orchestrator) ask(decision *Decision) (Result, error) {
	options, ok := optionsForParam(o.catalog, decision.Param)
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownParam, decision.Param)
	}

	return Result{
		Kind:     ResultKindAsk,
		Question: decision.Question,
		Param:    decision.Param,
		Options:  options,
	}, nil
}

// optionsForParam searches every endpoint's parameters and, for an unsafe
// endpoint, its request body's own properties, for the first one named
// name that declares an enum, and builds the options a person is offered
// from that schema's Enum and EnumLabels.
//
// The catalogue is authoritative here rather than Decision.Options: a
// Decision is ultimately produced by a model (docs/plans/orchestration.md
// Task 9), which can name candidate values that are not real, so an ask
// result must only ever offer values the catalogue itself declares for
// that parameter. The second return value is false when no endpoint
// declares name as an enum at all, which Plan reports as ErrUnknownParam
// rather than guessing or falling back to the model's own list.
func optionsForParam(catalog domain.Catalog, name string) ([]domain.Option, bool) {
	for i := range catalog.Endpoints {
		e := &catalog.Endpoints[i]

		for j := range e.Parameters {
			p := &e.Parameters[j]
			if p.Name == name && len(p.Schema.Enum) > 0 {
				return optionsFromSchema(&p.Schema), true
			}
		}

		if e.RequestBody != nil && e.RequestBody.Type == domain.SchemaTypeObject {
			if schema, ok := e.RequestBody.Properties[name]; ok && len(schema.Enum) > 0 {
				return optionsFromSchema(&schema), true
			}
		}
	}

	return nil, false
}

// optionsFromSchema builds the options for one enum schema, in enum order.
// A value with no entry in EnumLabels - not something a spec-conformant
// service can produce, since the harness's own x-enum-labels lint requires
// every enum value to have one, but not ruled out by the type system - gets
// an empty label rather than being dropped, the same defensive choice
// tools.go's enumLabels makes for the model-facing description.
func optionsFromSchema(s *domain.Schema) []domain.Option {
	options := make([]domain.Option, len(s.Enum))
	for i, value := range s.Enum {
		options[i] = domain.Option{Value: value, Label: s.EnumLabels[value]}
	}

	return options
}

// invokeAndRender calls endpoint with args and renders the decoded
// response, shared by call's safe path and Invoke.
//
// The component is chosen by domain.RenderResult, not domain.Render: a
// request body means "unsafe, show a form instead of calling" to an
// endpoint that has not been called, and nothing at all to one that has.
// By the time this function renders, the call has been made, so the answer
// is drawn from the response schema.
func (o *Orchestrator) invokeAndRender(
	ctx context.Context,
	endpoint *domain.Endpoint,
	service, operationID string,
	args map[string]any,
) (Result, error) {
	data, err := o.invoker.Invoke(ctx, endpoint, args)
	if err != nil {
		return Result{}, fmt.Errorf("invoking %s/%s: %w", service, operationID, err)
	}

	return Result{
		Kind:        ResultKindResult,
		Component:   domain.RenderResult(endpoint),
		Data:        data,
		Service:     service,
		OperationID: operationID,
		Args:        args,
	}, nil
}

// validateArgs checks args against endpoint's schema before it is called:
// every required request body field must be present, and every enum-typed
// parameter or body property, when given a value, must use one of its
// declared values. This is not full JSON Schema validation - just the
// minimum the spec calls for (docs/plans/orchestration.md, Task 8) - so a
// wrong type or an unknown extra field is not caught here.
func validateArgs(endpoint *domain.Endpoint, args map[string]any) error {
	if endpoint.RequestBody != nil {
		for _, name := range endpoint.RequestBody.Required {
			if _, ok := args[name]; !ok {
				return fmt.Errorf("%w: missing required field %q", ErrInvalidArguments, name)
			}
		}

		for name, schema := range endpoint.RequestBody.Properties {
			if err := validateEnumArg(name, &schema, args); err != nil {
				return err
			}
		}
	}

	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]
		if err := validateEnumArg(p.Name, &p.Schema, args); err != nil {
			return err
		}
	}

	return nil
}

// validateEnumArg checks, when schema declares an enum and args carries a
// value for name, that the value is one of the declared ones.
func validateEnumArg(name string, schema *domain.Schema, args map[string]any) error {
	if len(schema.Enum) == 0 {
		return nil
	}

	value, ok := args[name]
	if !ok {
		return nil
	}

	given := fmt.Sprint(value)
	if slices.Contains(schema.Enum, given) {
		return nil
	}

	return fmt.Errorf("%w: %q is not a valid value for %q", ErrInvalidArguments, given, name)
}
