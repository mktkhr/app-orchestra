package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// PickKind is what the picker named for one question: an operation of the
// shortlist it was offered, or one of the three fixed lines the pick
// offers after it (docs/specs/staging.md, S3).
type PickKind string

// The four things a Picker can decide.
const (
	PickOperation        PickKind = "operation"
	PickListCapabilities PickKind = "list_capabilities"
	PickProposePanel     PickKind = "propose_panel"
	PickNone             PickKind = "none"
)

// Pick is what a Picker returns: one operation of the shortlist (or one of
// the three built-ins), and whether the pick itself judged the question
// ambiguous between candidates (S4).
type Pick struct {
	Kind PickKind
	// Service and OperationID are populated when Kind is PickOperation:
	// the shortlist endpoint's own Service and OperationID, so
	// Orchestrator.planStaged can look it up in the same narrowed
	// catalogue the pick was itself offered.
	Service     string
	OperationID string
	// Ambiguous is S4: recorded on the pick and, through
	// Orchestrator.planStaged's log line, on the measurement - never
	// acted on (asking on ambiguity is the lever v3-ask-on-collision
	// measured at 68 -> 56, docs/specs/staging.md section 2).
	Ambiguous bool
	// Confidence is Jev's own judged confidence for this "pick" answer
	// (internal/adapter/planner/jev's mapAnswer, read back from the
	// wire answer's own confidence field) - zero for the local picker
	// (internal/adapter/planner/pick), which has no confidence signal
	// of its own to report. Added for internal/adapter/planner/hybrid's
	// own confidence gate: it falls back from Jev to the local picker
	// exactly when this is below its own threshold, rather than acting
	// on Ambiguous (already spoken for by S4's own "never acted on"
	// rule above).
	Confidence float64
}

// Picker names one operation of the shortlist for the question, in the
// measured picker's format (docs/specs/staging.md, section 4), or one of
// the three fixed lines S3 offers beside it. Implemented by
// internal/adapter/planner/pick over chat.Client - the usecase layer
// itself imports neither net/http nor encoding/json (S7).
//
// answers carries whatever the person has already answered - most often a
// reply to a rule 2 ask (askDegrade, orchestrator_ask.go) surfaced on a
// previous pick - so a re-plan's pick step can read it too, not just the
// fill that follows a pick (docs/specs/staging.md, section 4, added
// 2026-09-16 alongside the ask_user degradation fix): before this, a pick
// re-run after an answer saw only the raw query again, with no memory of
// what the person had already narrowed down. nil/empty is the ordinary
// case (no ask has happened yet), and pick/prompt.go's own user-message
// builder must produce byte-identical output to before whenever answers is
// empty.
//
// turns is the conversation before query, already truncated to
// Orchestrator.contextWindow the same way Planner.Plan's own turns is
// (orchestrator_staging.go's planStaged calls truncateTurns before either
// call) - the v5 Jev trial's own hypothesis (docs/measurements/jev-picker-v5.md):
// the fill (planPreferred) already receives turns, but before this the
// pick never did, so a follow-up naming no service or operation of its
// own ("勤怠でも同じことして") had nothing to resolve "同じこと" against
// at the pick stage. internal/adapter/planner/pick's Picker ignores turns
// entirely (its prompt, byte-identical to e2e/narrowing/pick/client.ts's
// own PICK_SYSTEM_PROMPT, has nothing to render them into - S2's
// cross-language comparison depends on that staying true);
// internal/adapter/planner/jev's Picker sends them inside its own request.
type Picker interface {
	Pick(ctx context.Context, query string, answers []Answer, turns []Turn, shortlist domain.Catalog) (Pick, error)
}
