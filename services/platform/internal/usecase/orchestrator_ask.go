package usecase

import (
	"slices"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

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
// A safe operation whose param has no catalogue enum at all no longer
// degrades unconditionally to an empty form (fixed 2026-09-16, TODO.md item
// 3): askDegrade decides between a form, a model-supplied list of options,
// or a plain question - see its own doc comment for the three-way rule.
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
		return askDegrade(&endpoint, decision), nil
	}

	return Result{
		Kind:     ResultKindAsk,
		Question: decision.Question,
		Param:    decision.Param,
		Options:  options,
	}, nil
}

// askDegrade decides what a safe ask_user decision becomes when
// optionsForParam found no catalogue enum for decision.Param (decided
// 2026-09-16, DECISIONS.md, fixing TODO.md item 3: 「注文を見たい」 no
// longer renders listSalesOrders' empty filter form in place of 受注です
// か、発注ですか). Three cases, tried in this order:
//
//  1. decision.Param names a required argument of endpoint - the same
//     "required" list requiredParamsKnown and inputSchemaFor build
//     (requiredParamNames, tools.go). A required free-text field, such as
//     "name" on a create, is exactly what a form's text input is for:
//     typing an id or a name is not something a list of options could
//     offer instead, so today's form behaviour is kept unchanged.
//  2. Otherwise, when the model's own decision.Options names two or more
//     candidates, they are trusted and returned verbatim as a
//     ResultKindAsk (an empty Label falls back to its Value,
//     optionsWithLabelFallback). This is the one place ask ever prefers
//     the model's own Options over the catalogue's - contrast
//     optionsForParam's doc comment, where the catalogue is authoritative
//     because a model can invent values that are not real. That reasoning
//     does not apply here: there is no catalogue enum for this param at
//     all, so there is no catalogue-authoritative list to prefer instead,
//     and a param that is optional (not case 1) with two or more concrete
//     candidates the model already named - 受注 or 発注, say - is a real
//     question with real answers, not a guess. The person's pick comes
//     back exactly as an enum ask's already does - answers:
//     [{param, value}] - because Param is carried onto the result
//     unchanged.
//  3. Otherwise (no options, or exactly one) a plain question: Param and
//     Options are both left zero-valued, so the person answers in their
//     next message rather than picking from a list of one, and that reply
//     reaches the planner as a Turn (see planOrdinary/planStaged), the
//     same path an unresolved-endpoint ask already uses above.
//
// endpoint is a pointer for the same gocritic hugeParam reason as ask's own
// decision parameter (domain.Endpoint is 136 bytes; see
// harness/quality/go/golangci.yml).
func askDegrade(endpoint *domain.Endpoint, decision *Decision) Result {
	if paramIsRequired(endpoint, decision.Param) {
		return formFor(endpoint, decision)
	}

	if len(decision.Options) >= minAskOptions {
		return Result{
			Kind:     ResultKindAsk,
			Question: decision.Question,
			Param:    decision.Param,
			Options:  optionsWithLabelFallback(decision.Options),
		}
	}

	return Result{Kind: ResultKindAsk, Question: decision.Question}
}

// minAskOptions is the fewest model-supplied options askDegrade's second
// case will trust as a real choice (see its doc comment): one option is not
// a choice, so mnd (part of the fixed harness policy,
// harness/quality/go/golangci.yml) would otherwise flag the bare literal.
const minAskOptions = 2

// paramIsRequired reports whether name is a required argument of endpoint,
// reading the same list requiredParamsKnown does (requiredParamNames,
// tools.go) so askDegrade's case 1 can never disagree with what a form
// would have shown as required anyway.
func paramIsRequired(endpoint *domain.Endpoint, name string) bool {
	return slices.Contains(requiredParamNames(endpoint), name)
}

// optionsWithLabelFallback copies the model's own ask_user options,
// filling in an empty Label with its Value - the same defensive fallback
// optionsFromSchema applies when a catalogue enum is missing an
// x-enum-labels entry, kept as a separate copy here (rather than shared)
// because the two draw from different sources - the model's own Decision
// here, the catalogue's schema there - even though the shape they produce
// is identical.
func optionsWithLabelFallback(options []domain.Option) []domain.Option {
	out := make([]domain.Option, len(options))

	for i, o := range options {
		label := o.Label
		if label == "" {
			label = o.Value
		}

		out[i] = domain.Option{Value: o.Value, Label: label}
	}

	return out
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
