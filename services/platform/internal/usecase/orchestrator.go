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

// The five outcomes /api/plan can report (docs/specs/proposing.md, section
// 4, adds ResultKindProposal beside the four orchestration.md already
// settled).
const (
	ResultKindResult   ResultKind = "result"
	ResultKindForm     ResultKind = "form"
	ResultKindAsk      ResultKind = "ask"
	ResultKindNone     ResultKind = "none"
	ResultKindProposal ResultKind = "proposal"
)

// Result is what Orchestrator.Plan returns: the outcome of one question,
// shaped so a handler can render it directly onto the /api/plan response.
type Result struct {
	Kind ResultKind

	// Populated when Kind is ResultKindResult or ResultKindForm: which
	// endpoint the result came from, or the form would submit to.
	Service string
	// ServiceDisplayName is Service's own display name -
	// endpoint.ServiceDisplayNameOr(Service) - carried alongside it for
	// the same reason DisplayName is carried alongside OperationID one
	// level up (DECISIONS.md, 2026-09-13): Provenance.tsx reads this,
	// never Service.
	ServiceDisplayName string
	OperationID        string

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
	// View carries the result's chart axes when Kind is ResultKindResult
	// and the endpoint's contract declared x-ui-hint.chart
	// (docs/specs/dashboard.md, P2): an answer draws as a chart with
	// nothing configured. Only View.Chart is ever set here - a contract
	// declares axes, never a transform - and nil when the endpoint
	// declared no chart hint.
	View *domain.View

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

	// Title is populated when Kind is ResultKindProposal: the panel's
	// title, the model's own or the operation's display name (see
	// propose). Component, Args and View above are reused for a
	// proposal's own panel - the same fields a ResultKindResult already
	// carries, since a proposal is a plan result with a panel attached,
	// not a distinct shape (docs/specs/proposing.md, N2).
	Title string

	// Alternatives is populated when Kind is ResultKindResult, narrowing
	// is on, and the chosen operation was found at a position in the
	// narrowed shortlist: the shortlist's own next up-to-two entries,
	// after the chosen one (docs/specs/shortlisting.md, section 4,
	// AC-H-103). nil - not the top of the shortlist, and never populated
	// at all with PassThroughNarrower or when the request set Preferred
	// (see alternativesFor).
	Alternatives []Alternative
}

// Alternative is one further candidate offered beside a ResultKindResult,
// taken from the narrowed shortlist rather than the planner's own opinion
// (docs/specs/shortlisting.md, H5): choosing one re-plans with Preferred
// set to its OperationID.
type Alternative struct {
	OperationID string
	DisplayName string
	Service     string
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

// ErrToolNotOffered is returned when a Decision resolves to a built-in
// tool (BuiltinTool) whose condition ToolsFor(catalog, planCtx) evaluated
// false for this request - propose_panel called from a question with no
// workspace id, today. O5 (docs/specs/offering.md): the list is what the
// model was offered, not what the platform trusts, so a call to a tool
// that was not on it is refused the same way an unknown operation is
// (ErrEndpointNotFound) - a distinct sentinel because nothing here names a
// catalogue endpoint at all.
var ErrToolNotOffered = errors.New("tool not offered for this request")

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
	narrower      Narrower
	narrowK       int
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

// WithNarrower overrides the default PassThroughNarrower: Plan calls
// n.Narrow(ctx, catalog, query, k) after catalogFor and before ToolsFor
// (docs/specs/shortlisting.md, H2). pkg/app passes this only when
// ORCHESTRA_NARROWING_* is fully configured (H7); every other caller,
// including every test that predates this option, keeps the pass-through
// and so keeps offering the planner exactly what it always has (AC-H-101).
func WithNarrower(n Narrower, k int) Option {
	return func(o *Orchestrator) {
		o.narrower = n
		o.narrowK = k
	}
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
		narrower:      PassThroughNarrower{},
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
//
// workspaceID is what the browser's POST /api/plan carries in the same
// field (docs/specs/offering.md, O4): "" for a question asked from the
// chat screen, which has no workspace to put a panel on, and the
// workspace's own id otherwise. It becomes the PlanContext every
// BuiltinTool's own condition is evaluated against (O2) - here, and only
// here, once per request - so the tool list o.planner is offered, and the
// list a later DecisionProposal is checked against (see the ErrToolNotOffered
// check below), can never disagree.
//
// preferred is PlanRequest.preferred (docs/specs/shortlisting.md, section
// 4): "" for an ordinary question, or an operation id when the person
// chose one of a previous result's Alternatives. When set, o.narrower is
// never called at all - the catalogue becomes exactly that one operation,
// looked up in the permission-filtered catalogue catalogFor already
// returned, so an id the person may not call is ErrEndpointNotFound, the
// same as an unknown one - and the rest of the decision is delegated to
// planPreferred, which no longer offers the built-ins alongside it (see
// planPreferred's own doc comment for why).
//
// thinking is PlanRequest.thinking (docs/specs/shortlisting.md, "platform
// knobs" subproject, decided 2026-09-16): nil for a caller that leaves the
// platform's configured default alone, or the value the question was
// asked with, carried unchanged to o.planner.Plan on both the ordinary
// path below and planPreferred's.
func (o *Orchestrator) Plan(
	ctx context.Context, user *domain.User, query string, answers []Answer, turns []Turn, workspaceID, preferred string,
	thinking *bool,
) (Result, error) {
	catalog, err := o.catalogFor(ctx, user)
	if err != nil {
		return Result{}, err
	}

	if preferred != "" {
		endpoint, ok := findByOperationID(catalog, preferred)
		if !ok {
			return Result{}, fmt.Errorf("%w: %s", ErrEndpointNotFound, preferred)
		}

		return o.planPreferred(ctx, &endpoint, query, answers, turns, thinking)
	}

	// Narrowed to a shortlist before the planner ever sees it
	// (docs/specs/shortlisting.md, H1/H2): o.narrower is
	// PassThroughNarrower by default (NewOrchestrator), which returns
	// catalog unchanged, so this is a no-op until pkg/app configures
	// a real one. catalog is kept narrowed for the rest of Plan -
	// call, ask, listCapabilities and propose below all read this
	// same, already-narrowed value, the same way they always read
	// catalogFor's single permission-narrowed value.
	catalog, err = o.narrower.Narrow(ctx, catalog, query, o.narrowK)
	if err != nil {
		return Result{}, fmt.Errorf("narrowing catalogue: %w", err)
	}

	planCtx := PlanContext{WorkspaceID: workspaceID}
	tools := ToolsFor(catalog, planCtx)

	decision, err := o.planner.Plan(ctx, query, answers, truncateTurns(turns, o.contextWindow), tools, thinking)
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
	case DecisionProposal:
		if !toolOffered(tools, ProposePanelToolName) {
			return Result{}, fmt.Errorf("%w: %s", ErrToolNotOffered, ProposePanelToolName)
		}

		return o.propose(catalog, &decision)
	default:
		return Result{}, fmt.Errorf("%w: unknown decision kind %q", ErrNotImplemented, decision.Kind)
	}
}

// toolOffered reports whether name is one of tools - the list ToolsFor
// built for this one request, which may have excluded a BuiltinTool whose
// condition planCtx did not satisfy (O2). See ErrToolNotOffered.
func toolOffered(tools []Tool, name string) bool {
	for i := range tools {
		if tools[i].Name == name {
			return true
		}
	}

	return false
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

	if err := validateArgs(&endpoint, args); err != nil {
		return Result{}, err
	}

	return o.invokeAndRender(ctx, &endpoint, service, operationID, args)
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

// catalogFor narrows o.catalog to what user may call, through the shared
// catalogFor (auth.go): Plan and Invoke each call it exactly once and pass
// the result to every helper that reads the catalogue for that request
// (call, ask, listCapabilities, Find), so the tool list a planner is
// offered and the operation /api/invoke will run are read from the same
// narrowed value and cannot disagree.
func (o *Orchestrator) catalogFor(ctx context.Context, user *domain.User) (domain.Catalog, error) {
	return catalogFor(ctx, o.catalog, o.permissions, user)
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
//
// A safe call's result carries Alternatives (see alternativesFor); an
// unsafe one's form does not - a form is confirmed, not chosen among, and
// AC-H-103 only ever speaks of "a result".
func (o *Orchestrator) call(ctx context.Context, catalog domain.Catalog, decision *Decision) (Result, error) {
	endpoint, ok := catalog.Find(decision.Service, decision.OperationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, decision.Service, decision.OperationID)
	}

	if !endpoint.IsSafe() {
		return formFor(&endpoint, decision), nil
	}

	result, err := o.invokeAndRender(ctx, &endpoint, decision.Service, decision.OperationID, decision.Args)
	if err != nil {
		return Result{}, err
	}

	result.Alternatives = o.alternativesFor(catalog, decision.Service, decision.OperationID)

	return result, nil
}

// alternativesFor reads the shortlist positions after the one the chosen
// operation sits at, up to two (docs/specs/shortlisting.md, section 4,
// AC-H-103) - not the top of the shortlist, which alternativesFor never
// even looks at.
//
// It returns nil in every case a shortlist position is meaningless:
// PassThroughNarrower (H7 - the wire stays byte-identical to today, and
// that includes carrying no alternatives key at all) short-circuits
// before the catalogue is even searched; a built-in tool never reaches
// call at all, so its result never reaches this function either; and
// Preferred's one-endpoint catalogue does reach here, but the chosen
// operation sits at its only position with nothing after it, so the loop
// below finds none - AC-H-103's "preferred carries no alternatives"
// without a second flag to carry that decision.
func (o *Orchestrator) alternativesFor(catalog domain.Catalog, service, operationID string) []Alternative {
	if _, passThrough := o.narrower.(PassThroughNarrower); passThrough {
		return nil
	}

	idx := -1

	for i := range catalog.Endpoints {
		if catalog.Endpoints[i].Service == service && catalog.Endpoints[i].OperationID == operationID {
			idx = i

			break
		}
	}

	if idx < 0 {
		return nil
	}

	var alternatives []Alternative

	for i := idx + 1; i < len(catalog.Endpoints) && len(alternatives) < 2; i++ {
		e := &catalog.Endpoints[i]
		alternatives = append(alternatives, Alternative{
			OperationID: e.OperationID,
			DisplayName: e.DisplayNameOr(e.Summary),
			Service:     e.Service,
		})
	}

	return alternatives
}

// findByOperationID looks up the endpoint with the given operation id,
// regardless of service: operation ids are unique across a catalogue
// offered to a planner (the same assumption toolcall.Planner's
// resolveService makes to turn a tool call's name back into a service),
// which is what lets PlanRequest.preferred name an operation without
// naming its service too.
func findByOperationID(catalog domain.Catalog, operationID string) (domain.Endpoint, bool) {
	for i := range catalog.Endpoints {
		if catalog.Endpoints[i].OperationID == operationID {
			return catalog.Endpoints[i], true
		}
	}

	return domain.Endpoint{}, false
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
// An ask naming an unsafe operation degrades to that same form before the
// parameter is even looked at, whether or not it has an enum
// (docs/specs/orchestration.md, D11 and section 8b): D8 already answers an
// unsafe call with a form in every other case, that form already carries
// the parameter as a select over the same enum, and asking the question
// again first would only make the person answer it twice. An ask over a
// safe operation is unaffected - it still runs on whatever value the model
// asks about, so an enum found in the catalogue is offered as a
// ResultKindAsk exactly as before.
//
// decision is a pointer for the same reason call's is: golangci-lint's
// gocritic hugeParam check on Decision's 112 bytes (see
// harness/quality/go/golangci.yml).
//
// When decision.Service/decision.OperationID name no endpoint in catalog
// at all - absent, or invented (measured 2026-09-15,
// docs/specs/shortlisting.md: a five-service catalogue tempts the model
// into fabricating a service such as "approval" or "salesBundle", or
// naming its own tool as the operation, "expense/ask_user") - ask degrades
// to a plain question instead of ErrEndpointNotFound: an ask is never a
// 500, it just has nothing left to offer beyond the question itself. This
// is deliberately here, in the usecase, rather than in either planner
// adapter, so both internal/adapter/planner/toolcall and
// internal/adapter/planner/jsonmode get it for free.
func (o *Orchestrator) ask(catalog domain.Catalog, decision *Decision) (Result, error) {
	endpoint, ok := catalog.Find(decision.Service, decision.OperationID)
	if !ok {
		return Result{Kind: ResultKindAsk, Question: decision.Question}, nil
	}

	if !endpoint.IsSafe() {
		return formFor(&endpoint, decision), nil
	}

	options, ok := optionsForParam(&endpoint, decision.Param)
	if !ok {
		return formFor(&endpoint, decision), nil
	}

	return Result{
		Kind:     ResultKindAsk,
		Question: decision.Question,
		Param:    decision.Param,
		Options:  options,
	}, nil
}

// propose resolves a DecisionProposal into a ResultKindProposal: it never
// touches the invoker, and never checks whether the endpoint is safe -
// unlike call, a proposal is never run at all, whether the operation it
// names is safe or not (N1, docs/specs/proposing.md), so there is nothing
// here for D8's safe/unsafe split to decide between.
//
// The catalogue lookup is the same ErrEndpointNotFound every other
// unknown-or-forbidden operation produces (see call, ask): a proposal
// naming an operation the person may not call cannot be produced, because
// catalog here is already the caller's narrowed one (catalogFor) - the
// same reason Invoke's own lookup answers a forbidden operation with
// ErrEndpointNotFound rather than a distinct "forbidden" sentinel
// (docs/specs/auth.md, A4; AC-N-105).
//
// Every optional field the model left zero-valued is filled in from the
// catalogue, never from decision.Args or a second guess - see
// proposalComponent, proposalView and proposalTitle for what each of them
// reads (section 4: "The platform fills in what the model left out").
//
// decision is a pointer for the same gocritic hugeParam reason as call's
// and ask's (Decision is over 100 bytes; see harness/quality/go/golangci.yml).
func (o *Orchestrator) propose(catalog domain.Catalog, decision *Decision) (Result, error) {
	endpoint, ok := catalog.Find(decision.Service, decision.OperationID)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, decision.Service, decision.OperationID)
	}

	return Result{
		Kind:               ResultKindProposal,
		Service:            decision.Service,
		ServiceDisplayName: endpoint.ServiceDisplayNameOr(decision.Service),
		OperationID:        decision.OperationID,
		Args:               decision.Args,
		Component:          proposalComponent(&endpoint, decision),
		View:               proposalView(&endpoint, decision),
		Title:              proposalTitle(&endpoint, decision),
	}, nil
}

// proposalComponent answers what a proposal draws its panel with: the
// model's own Component when it named one, otherwise domain.Render's own
// rule - the endpoint has not been called and may never be (N1), exactly
// the situation Render (not RenderResult) already answers for a call the
// platform is choosing between.
func proposalComponent(endpoint *domain.Endpoint, decision *Decision) domain.Component {
	if decision.Component != "" {
		return decision.Component
	}

	return domain.Render(endpoint)
}

// proposalView merges the model's own view with the catalogue's, one half
// at a time: a Chart the model gave wins outright, and one it left out is
// filled from the endpoint's own x-ui-hint.chart when the contract
// declares one (chartViewFor); Transform has no catalogue-sourced default
// at all - grouping is the model's own judgment about the question, or
// nothing - so it is only ever the model's own. Returns nil when neither
// half ends up set, exactly as chartViewFor's own callers already expect
// (AC-P-106's convention: no view, not an empty one).
func proposalView(endpoint *domain.Endpoint, decision *Decision) *domain.View {
	var transform *domain.Transform

	chart := endpoint.ChartHint

	if decision.View != nil {
		transform = decision.View.Transform

		if decision.View.Chart != nil {
			chart = decision.View.Chart
		}
	}

	if chart == nil && transform == nil {
		return nil
	}

	return &domain.View{Chart: chart, Transform: transform}
}

// proposalTitle answers a proposal's title: the model's own when it gave
// one, otherwise the operation's display name - the same DisplayNameOr
// fallback (to the operation id) CreatePanelRequest's own "title" already
// documents for a panel saved with none.
func proposalTitle(endpoint *domain.Endpoint, decision *Decision) string {
	if decision.Title != "" {
		return decision.Title
	}

	return endpoint.DisplayNameOr(decision.OperationID)
}

// formFor builds the form the platform hands a person instead of running
// something: the endpoint's whole argument schema, and whatever arguments
// the model did manage to fill in as its initial values.
//
// All three places that produce one are the same idea, which is why they
// share this function rather than each writing the literal out. Two of them
// are the same rule twice over - an unsafe operation is answered by its
// form, whether the model tried to call it (call) or reached for ask_user
// on an argument it could not fill (ask), because D8 says the model never
// runs an unsafe operation at all and a person pressing the button is what
// does (docs/specs/orchestration.md, section 8b). The third is D11's own
// degradation: a safe operation whose stuck argument has no enum has no
// list of values to offer, and letting the person type it is what a form
// is for.
func formFor(endpoint *domain.Endpoint, decision *Decision) Result {
	return Result{
		Kind:               ResultKindForm,
		Service:            decision.Service,
		ServiceDisplayName: endpoint.ServiceDisplayNameOr(decision.Service),
		OperationID:        decision.OperationID,
		Schema:             inputSchemaFor(endpoint),
		Initial:            decision.Args,
	}
}

// optionsForParam searches one endpoint's parameters for the one named
// name that declares an enum, and builds the options a person is offered
// from that schema's Enum and EnumLabels. Only a safe endpoint's ask ever
// reaches this function - ask degrades an unsafe endpoint to a form before
// looking at the parameter at all (see ask) - so there is no request body
// to search: a request body only ever appears on an unsafe endpoint's
// call.
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
		Kind: ResultKindResult,
		// "platform" is not a configured service - list_capabilities is
		// the platform's own built-in tool (D14, docs/specs/orchestration.md)
		// - so there is no contract to read a display name from, and none
		// to fall back from either: ServiceDisplayName just repeats
		// Service, exactly as this Provenance already read before
		// ServiceDisplayName existed.
		Service:            "platform",
		ServiceDisplayName: "platform",
		OperationID:        "list_capabilities",
		Component:          domain.ComponentTable,
		Data:               map[string]any{"items": capabilitiesItems(catalog, decision.Service)},
		Fields:             capabilitiesFields(),
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
			paramService: e.ServiceDisplayNameOr(e.Service),
			"operation":  e.DisplayNameOr(e.OperationID),
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
		Kind:               ResultKindResult,
		Component:          domain.RenderResult(endpoint),
		Data:               data,
		Service:            service,
		ServiceDisplayName: endpoint.ServiceDisplayNameOr(service),
		OperationID:        operationID,
		Args:               args,
		Fields:             fieldsFor(endpoint),
		View:               chartViewFor(endpoint),
	}, nil
}

// chartViewFor carries an endpoint's contract-declared chart axes onto its
// result's View, and nil when the contract declares none. Only the chart
// half is ever set: a contract declares axes, never a transform
// (docs/plans/dashboard.md, Task 3, "The shape everything shares").
func chartViewFor(endpoint *domain.Endpoint) *domain.View {
	if endpoint.ChartHint == nil {
		return nil
	}

	return &domain.View{Chart: endpoint.ChartHint}
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
