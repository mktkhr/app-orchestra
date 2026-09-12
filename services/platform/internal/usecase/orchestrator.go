package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

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
	// Fields describes, per property, the schema a table's columns or a
	// detail's own properties carry - most importantly each enum
	// property's EnumLabels, so a UI can show "検品保留" instead of
	// "quarantined" without parsing the model-facing description string
	// apart (see DECISIONS.md). Built by fieldsFor, and nil whenever
	// domain.FieldsSchema finds nothing to describe (Component is not
	// table or detail).
	Fields map[string]any

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

// messageNoEndpoint is the message a ResultKindNone result carries: the
// planner itself decided nothing in the catalogue fits the question. It
// points at list_capabilities rather than leaving the person at a dead
// end - a genuinely unanswerable question ("今日の天気は？") still lands
// here, but so does someone who simply does not know what to ask yet, and
// the second case has an answer the first does not.
const messageNoEndpoint = "その質問に答えられる操作が見つかりませんでした。" +
	"「何ができるの？」と聞くと、できることの一覧を確認できます。"

// DefaultContextWindow is the number of turns Orchestrator keeps when no
// Option overrides it via WithContextWindow - pkg/app always does, from
// ORCHESTRA_CONTEXT_TURNS, so this constant is only ever reached by a
// caller (a test, mainly) that does not care about the window. Chosen
// small on purpose (docs/specs/context.md, section 6): a handful of
// follow-up questions is what a conversation about one task actually
// needs, and every turn kept is a turn that must be rendered into the
// prompt on the very next question.
const DefaultContextWindow = 8

// Orchestrator drives one /api/plan request: it asks Planner for a
// Decision over the catalogue's tools and, for a safe call, invokes it and
// renders the result (docs/specs/orchestration.md, section 4).
type Orchestrator struct {
	catalog       domain.Catalog
	planner       Planner
	invoker       Invoker
	permissions   PermissionStore
	contextWindow int
}

// Option configures an Orchestrator built by NewOrchestrator, beyond its
// required collaborators.
type Option func(*Orchestrator)

// WithContextWindow overrides DefaultContextWindow: Plan keeps only the
// most recent n turns of what it is given, oldest dropped first
// (docs/specs/context.md, section 6). pkg/app always passes this, from
// ORCHESTRA_CONTEXT_TURNS - it is an Option, rather than a required
// NewOrchestrator parameter, so the many existing callers that do not care
// about the window did not all have to change to add it.
func WithContextWindow(n int) Option {
	return func(o *Orchestrator) { o.contextWindow = n }
}

// NewOrchestrator builds an Orchestrator over the given catalogue, planner,
// invoker and permission store. permissions is read once per request, by
// catalogFor, to narrow catalog down to what the calling person may call
// (docs/specs/auth.md, section 5) - never consulted for an admin, who holds
// every permission implicitly (docs/specs/auth.md, section 4).
func NewOrchestrator(
	catalog domain.Catalog, planner Planner, invoker Invoker, permissions PermissionStore, opts ...Option,
) *Orchestrator {
	o := &Orchestrator{
		catalog:       catalog,
		planner:       planner,
		invoker:       invoker,
		permissions:   permissions,
		contextWindow: DefaultContextWindow,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Plan turns a question (plus any answers to a previous ask, and the
// conversation before it) into a Result: a safe call is invoked and
// rendered, an unsafe call comes back as a form to confirm, an ask
// decision comes back as a disambiguation question built from the
// catalogue (see ask), and none reports that nothing fits.
//
// turns is the conversation as the browser holds it, oldest first
// (docs/specs/context.md, section 3); Plan truncates it to the most recent
// o.contextWindow entries, oldest dropped first, before it ever reaches
// the planner (section 6) - the browser sends what it has and the
// platform is what knows the model, so the platform is what decides how
// much of it the model is told. Empty or nil turns - a question with no
// history - behaves exactly as it did before turns existed (AC-M-105):
// truncateTurns returns it unchanged and no planner reads turns yet
// (docs/plans/context.md, Task 1).
//
// user is not context.Context because a value in a context is one a
// caller can forget to put there, and the failure that produces is a
// permission check that silently passes; an argument cannot be forgotten,
// because the code does not compile without it (docs/specs/auth.md,
// section 5, A6).
func (o *Orchestrator) Plan(
	ctx context.Context, user *domain.User, query string, answers []Answer, turns []Turn,
) (Result, error) {
	catalog, err := o.catalogFor(ctx, user)
	if err != nil {
		return Result{}, err
	}

	tools := ToolsFor(catalog)

	decision, err := o.planner.Plan(ctx, query, answers, truncateTurns(turns, o.contextWindow), tools)
	if err != nil {
		return Result{}, fmt.Errorf("planning: %w", err)
	}

	switch decision.Kind {
	case DecisionNone:
		return Result{Kind: ResultKindNone, Message: messageNoEndpoint}, nil
	case DecisionCall:
		return o.call(ctx, catalog, &decision)
	case DecisionAsk:
		return o.ask(catalog, &decision)
	case DecisionListCapabilities:
		return o.listCapabilities(catalog, &decision), nil
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
// The catalogue lookup is against catalogFor(ctx, user), not o.catalog: an
// operation the person may not call is not found here any more than an
// operation that does not exist at all is (docs/specs/auth.md, section 5;
// AC-A-104) - the same ErrEndpointNotFound, so a 403 never tells anybody
// what exists.
func (o *Orchestrator) Invoke(ctx context.Context, user *domain.User, service, operationID string, args map[string]any) (Result, error) {
	catalog, err := o.catalogFor(ctx, user)
	if err != nil {
		return Result{}, err
	}

	endpoint, ok := catalog.Find(service, operationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, service, operationID)
	}

	args = stripSyntheticAll(&endpoint, args)

	if err := validateArgs(&endpoint, args); err != nil {
		return Result{}, err
	}

	return o.invokeAndRender(ctx, &endpoint, service, operationID, args)
}

// stripSyntheticAll removes any argument named after one of endpoint's own
// enum parameters whose value is domain.EnumAllValue - the synthetic value
// D15 (docs/specs/orchestration.md, section 8a) adds to a safe endpoint's
// optional enum parameter so the model must say it rather than omit the
// parameter. It is called after the catalogue lookup and before validation,
// the call, or the result's provenance are built, on both paths that reach
// a service (call's safe branch and Invoke) - so a service never learns the
// value exists, source.args reads {} exactly as it did when the model
// omitted the parameter, and a workspace panel saved from the result holds
// no __all__ to replay.
//
// args itself is never mutated: it may be decision.Args, which the caller
// (call, for an unsafe endpoint) also uses as a form's Initial values, and
// mutating a shared map out from under that use would be a surprise a
// reader of either call site should never have to rule out.
func stripSyntheticAll(endpoint *domain.Endpoint, args map[string]any) map[string]any {
	if len(args) == 0 {
		return args
	}

	stripped := make(map[string]any, len(args))

	for name, value := range args {
		if value == domain.EnumAllValue && isEnumParam(endpoint, name) {
			continue
		}

		stripped[name] = value
	}

	return stripped
}

// isEnumParam reports whether name is one of endpoint's own parameters (not
// a request body property - D15 never offers the synthetic value there)
// that declares an enum, which is the only kind of argument
// stripSyntheticAll ever removes.
func isEnumParam(endpoint *domain.Endpoint, name string) bool {
	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]
		if p.Name == name && len(p.Schema.Enum) > 0 {
			return true
		}
	}

	return false
}

// truncateTurns keeps the most recent window entries of turns, oldest
// dropped first (docs/specs/context.md, section 6). turns is oldest first
// (Plan's own doc comment), so the ones to keep are its tail.
//
// window <= 0 is treated as "keep nothing" rather than "unbounded" - a
// misconfigured ORCHESTRA_CONTEXT_TURNS should shrink the window to
// nothing a person can notice, not silently turn it off and let a long
// conversation grow the prompt forever.
func truncateTurns(turns []Turn, window int) []Turn {
	if window <= 0 || len(turns) == 0 {
		return nil
	}

	if len(turns) <= window {
		return turns
	}

	return turns[len(turns)-window:]
}

// catalogFor narrows o.catalog to what user may call: the whole catalogue
// for an admin (docs/specs/auth.md, section 4 - "that is the whole of what
// the role buys" is about permissions, not this exception, but the row
// count it would otherwise take is exactly what seeding every permission
// for every admin would cost), or domain.Catalog.For(permissions) for
// anybody else.
//
// This is the one seat docs/specs/auth.md section 5 names: Plan and Invoke
// each call catalogFor exactly once, and pass the result to every helper
// that reads the catalogue for that request (call, ask, listCapabilities,
// Find) - so the tool list a planner is offered and the operation
// /api/invoke will run are read from the same narrowed value and cannot
// disagree.
func (o *Orchestrator) catalogFor(ctx context.Context, user *domain.User) (domain.Catalog, error) {
	if user.Role == domain.RoleAdmin {
		return o.catalog, nil
	}

	permissions, err := o.permissions.For(ctx, user.ID)
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("reading permissions for %s: %w", user.ID, err)
	}

	return o.catalog.For(permissions), nil
}

// call resolves a DecisionCall against the catalogue: a safe endpoint is
// invoked and rendered; an unsafe one (D8, docs/specs/orchestration.md)
// never reaches the service at all - it comes back as a form to confirm
// instead, carrying the endpoint's whole argument schema (parameters and
// request body alike, see inputSchemaFor) and the planner's arguments as
// initial values.
//
// decision is a pointer, not the value Plan holds, because Decision is 112
// bytes: golangci-lint's gocritic hugeParam check (part of the fixed
// harness policy, see harness/quality/go/golangci.yml) rejects passing it
// by value.
func (o *Orchestrator) call(ctx context.Context, catalog domain.Catalog, decision *Decision) (Result, error) {
	endpoint, ok := catalog.Find(decision.Service, decision.OperationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, decision.Service, decision.OperationID)
	}

	if !endpoint.IsSafe() {
		return Result{
			Kind:        ResultKindForm,
			Service:     decision.Service,
			OperationID: decision.OperationID,
			Schema:      inputSchemaFor(&endpoint),
			Initial:     decision.Args,
		}, nil
	}

	args := stripSyntheticAll(&endpoint, decision.Args)

	return o.invokeAndRender(ctx, &endpoint, decision.Service, decision.OperationID, args)
}

// ask resolves a DecisionAsk into a ResultKindAsk, or degrades it into a
// ResultKindForm: it never touches the invoker either way (an ask never
// calls a service), and it never repeats decision.Options verbatim - see
// optionsForParam for why the catalogue, not the model's own Decision, is
// the source of truth for what a person is offered to pick from.
//
// The endpoint is looked up from decision.Service and decision.OperationID
// - the operation the model was stuck on when it reached for ask_user
// instead - and decision.Param is only ever resolved against that one
// endpoint. A parameter name such as "status" or "type" is not unique
// across a catalogue of many services (docs/specs/orchestration.md, D11),
// so searching the whole catalogue by name alone can surface another
// service's enum entirely; naming the operation is what keeps the search
// inside the one endpoint the question is actually about.
//
// D11 designed ask_user around an enum: hand the choice back as a list of
// values to pick from. In practice the model also reaches for it when a
// required argument has no enum at all - most often a create's free-text
// field, such as "name" on CreateInventoryItem, when the question never
// said what to call the thing - because from the model's own point of
// view the situation is the same ("I cannot fill this in"), even though
// the catalogue has nothing to offer a list of. optionsForParam reports
// that with its second return value, and there is exactly one thing left
// to do with a value nobody can choose from a list: let the person type
// it, which is what a form is for. This is not a fallback that papers
// over an error - it is what the model was actually asking for, expressed
// through a tool that assumes an enum it did not have (see DECISIONS.md).
// decision.Args, when the model already filled in other arguments
// alongside the one it got stuck on, carries through as the form's
// initial values, exactly as an unsafe DecisionCall's do (see call).
//
// decision is a pointer for the same reason call's is: golangci-lint's
// gocritic hugeParam check on Decision's 112 bytes (see
// harness/quality/go/golangci.yml).
func (o *Orchestrator) ask(catalog domain.Catalog, decision *Decision) (Result, error) {
	endpoint, ok := catalog.Find(decision.Service, decision.OperationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, decision.Service, decision.OperationID)
	}

	options, ok := optionsForParam(&endpoint, decision.Param)
	if !ok {
		return Result{
			Kind:        ResultKindForm,
			Service:     decision.Service,
			OperationID: decision.OperationID,
			Schema:      inputSchemaFor(&endpoint),
			Initial:     decision.Args,
		}, nil
	}

	return Result{
		Kind:     ResultKindAsk,
		Question: decision.Question,
		Param:    decision.Param,
		Options:  options,
	}, nil
}

// optionsForParam searches one endpoint's parameters and, for an unsafe
// endpoint, its request body's own properties, for the one named name that
// declares an enum, and builds the options a person is offered from that
// schema's Enum and EnumLabels.
//
// The catalogue is authoritative here rather than Decision.Options: a
// Decision is ultimately produced by a model (docs/plans/orchestration.md
// Task 9), which can name candidate values that are not real, so an ask
// result must only ever offer values the catalogue itself declares for
// that parameter - and only for the one endpoint decision named, not
// whichever endpoint elsewhere in the catalogue happens to share the
// parameter's name. The second return value is false when this endpoint
// does not declare name as an enum at all - name may still be a real,
// required argument (a free-text field such as "name"), just not one with
// a list of values to offer - which ask reports by degrading to a form
// rather than guessing or falling back to the model's own list.
//
// endpoint is a pointer for the same gocritic hugeParam reason as call's
// and ask's decision parameter (domain.Endpoint is 136 bytes; see
// harness/quality/go/golangci.yml).
func optionsForParam(endpoint *domain.Endpoint, name string) ([]domain.Option, bool) {
	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]
		if p.Name == name && len(p.Schema.Enum) > 0 {
			return optionsFromSchema(&p.Schema), true
		}
	}

	if endpoint.RequestBody != nil && endpoint.RequestBody.Type == domain.SchemaTypeObject {
		if schema, ok := endpoint.RequestBody.Properties[name]; ok && len(schema.Enum) > 0 {
			return optionsFromSchema(&schema), true
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

// capabilitiesFields is the fixed Fields a list_capabilities result
// carries: it does not vary with the catalogue or the filter, unlike
// fieldsFor's output for a real endpoint, since list_capabilities' own
// output shape (service/operation/summary) is fixed by this function, not
// by any service's spec.
func capabilitiesFields() map[string]any {
	return map[string]any{
		paramService: map[string]any{keyType: domain.SchemaTypeString, keyTitle: "サービス"},
		"operation":  map[string]any{keyType: domain.SchemaTypeString, keyTitle: "操作"},
		"summary":    map[string]any{keyType: domain.SchemaTypeString, keyTitle: "できること"},
	}
}

// listCapabilities answers a DecisionListCapabilities entirely from the
// catalogue already held in memory: it never calls the invoker, unlike
// call and Invoke, because "what can this do" is a question about the
// catalogue itself, not about any one service's data. Building the answer
// from the catalogue - rather than asking the model to describe what it
// can do in prose - is deliberate: a model's own description of the
// catalogue can drift from it (naming an operation that does not exist, or
// missing one that does), and D8 (docs/specs/orchestration.md) already
// settled that a result comes from the API, not from the model's telling.
//
// decision is a pointer for the same gocritic hugeParam reason as call's
// and ask's (Decision is 112 bytes; see harness/quality/go/golangci.yml).
// catalog is the caller's already-narrowed catalogue (see catalogFor), the
// same reason call and ask take it rather than reading o.catalog: "what
// can this do" must answer from what the asking person may call, not from
// everything the platform has.
func (o *Orchestrator) listCapabilities(catalog domain.Catalog, decision *Decision) Result {
	return Result{
		Kind:        ResultKindResult,
		Service:     "platform",
		OperationID: "list_capabilities",
		Component:   domain.ComponentTable,
		Data:        map[string]any{"items": capabilitiesItems(catalog, decision.Service)},
		Fields:      capabilitiesFields(),
	}
}

// capabilitiesItems lists the catalogue's endpoints as
// service/operation/summary rows, filtered to service when it is
// non-empty. A service name that matches no endpoint (a typo, or a
// service that does not exist) yields an empty list rather than falling
// back to the unfiltered catalogue: the person asked about one service by
// name, and an empty table is an honest answer to "this service has
// nothing exposed", where silently substituting every service's
// operations would misrepresent what was asked.
//
// The result is sorted by (service, operationId) rather than left in
// catalogue order, so that "which order do the rows come out in" does not
// depend on how the operator listed services in configuration, or on
// anything iterating a map - domain.Catalog.Endpoints is itself already a
// plain slice, not a map, but sorting here makes the guarantee explicit
// and independent of how the catalogue happens to have been built.
func capabilitiesItems(c domain.Catalog, service string) []map[string]any {
	endpoints := make([]domain.Endpoint, 0, len(c.Endpoints))

	for i := range c.Endpoints {
		if service == "" || c.Endpoints[i].Service == service {
			endpoints = append(endpoints, c.Endpoints[i])
		}
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Service != endpoints[j].Service {
			return endpoints[i].Service < endpoints[j].Service
		}

		return endpoints[i].OperationID < endpoints[j].OperationID
	})

	items := make([]map[string]any, len(endpoints))

	for i := range endpoints {
		e := &endpoints[i]
		items[i] = map[string]any{
			paramService: e.Service,
			"operation":  e.OperationID,
			"summary":    e.Summary,
		}
	}

	return items
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
		Fields:      fieldsFor(endpoint),
	}, nil
}

// fieldsFor builds the per-property schema a ResultKindResult result
// carries as Fields: the row schema for a table, or the response schema
// itself for a detail - domain.FieldsSchema decides which, reusing exactly
// the judgment RenderResult already made about where a table's rows live
// (see soleArrayProperty), rather than repeating that logic here.
//
// The conversion to JSON Schema is schemaToJSONSchema, the same function
// tools.go uses to describe a schema to the model, so a property's enum
// labels are never derived twice. Only its "properties" map is kept -
// fieldsFor describes the fields themselves, not a wrapping object - and a
// schema with no properties (or none at all) yields no Fields, per
// docs/specs/orchestration.md section 6: absent, not an empty object.
func fieldsFor(endpoint *domain.Endpoint) map[string]any {
	rowSchema := domain.FieldsSchema(endpoint)
	if rowSchema == nil || len(rowSchema.Properties) == 0 {
		return nil
	}

	converted := schemaToJSONSchema(rowSchema)

	fields, ok := converted[keyProperties].(map[string]any)
	if !ok {
		return nil
	}

	return fields
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
