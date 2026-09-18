package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// Filler resolves an eligible pick's arguments without the local fill's own
// model call - ORCHESTRA_FILL_ENUM=jev's own port
// (internal/adapter/planner/jev, docs/measurements/jev-conditions.md: the
// fill, not the pick, owns end-to-end latency, and a closed-set argument is
// exactly the shape a coarse classifier answers well). Only ever consulted
// by planPreferred's fromPick path (orchestrator_preferred.go), and only
// for an endpoint EligibleForFillEnum reports true for.
//
// Fill returns the same Decision shape o.planner.Plan would have returned
// for this same operation - a DecisionCall naming the arguments every
// answered parameter resolved to (an unset parameter left out entirely,
// exactly as a value nobody supplied), or a DecisionAsk over the one
// parameter a mismatch was found on - fed straight into
// Orchestrator.resolvePickedFill exactly as if the local fill had produced
// it, so every existing safe/unsafe, ask and form rule there applies
// unchanged.
//
// ok is false whenever the caller must fail open: a transport error (also
// returned as err, logged by the caller), a missing answer, or any
// answer's confidence below the adapter's own configured threshold -
// never an error the caller must act on, and never ok true with a non-nil
// err. The adapter itself logs one info line per attempt (fill_provider,
// each parameter's chosen option and confidence, fill_ms,
// fill_input_tokens, fill_output_tokens, and the full probability
// distribution per question) whether or not it fails open, the same
// convention jev.Picker.Pick already logs pick_probabilities under.
type Filler interface {
	Fill(
		ctx context.Context, endpoint *domain.Endpoint, query string, answers []Answer, turns []Turn, planCtx PlanContext,
	) (Decision, bool, error)
}

// EligibleForFillSkip reports whether endpoint declares no parameters at
// all and needs no request body - ORCHESTRA_FILL_SKIP_EMPTY's own
// eligibility rule (arm 1, docs/measurements/jev-conditions.md): a picked
// operation nobody could have supplied an argument to anyway, safe or not,
// so the fill's own model call can only ever have named it with no
// arguments - the same outcome skipping it altogether produces directly.
func EligibleForFillSkip(endpoint *domain.Endpoint) bool {
	return len(endpoint.Parameters) == 0 && endpoint.RequestBody == nil
}

// EligibleForFillEnum reports whether endpoint declares no request body,
// at least one parameter, and every parameter is enum-valued
// (domain.Schema.Enum non-empty) - ORCHESTRA_FILL_ENUM=jev's own
// eligibility rule (arm 2): a free-text, number or date parameter has no
// closed set of options for a Choice question to offer, and a request body
// only ever appears on an unsafe create, which formFor already renders
// without ever asking the model to invent free text (see
// guessedEnumArgs's own doc comment, orchestrator_enum_guess.go, for why
// that scope is excluded there too). An endpoint with no parameters at all
// is EligibleForFillSkip's own case, not this one - arm 1 alone answers
// it, and there is no enum question for arm 2 to ask about an operation
// with nothing to fill in.
func EligibleForFillEnum(endpoint *domain.Endpoint) bool {
	if endpoint.RequestBody != nil || len(endpoint.Parameters) == 0 {
		return false
	}

	for i := range endpoint.Parameters {
		if len(endpoint.Parameters[i].Schema.Enum) == 0 {
			return false
		}
	}

	return true
}
