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
}

// Picker names one operation of the shortlist for the question, in the
// measured picker's format (docs/specs/staging.md, section 4), or one of
// the three fixed lines S3 offers beside it. Implemented by
// internal/adapter/planner/pick over chat.Client - the usecase layer
// itself imports neither net/http nor encoding/json (S7).
type Picker interface {
	Pick(ctx context.Context, query string, shortlist domain.Catalog) (Pick, error)
}
