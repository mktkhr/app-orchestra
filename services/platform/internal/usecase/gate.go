package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// GateVerdict is what a Gate returns for one question: whether it judges
// the question impossible for the shortlist it was given (no candidate
// operation, including every built-in the pick itself would offer, can
// satisfy it), and the raw probability that verdict came from - recorded
// so a caller can log it even when Impossible itself is what acts on it.
type GateVerdict struct {
	Impossible bool
	// Probability is the gate's own confidence that the question is
	// impossible (0..1) - Impossible is Probability compared against the
	// gate's own threshold, kept here separately so a caller (or a log
	// line) can see how close the call was, the same reason
	// usecase.Pick.Ambiguous is recorded alongside the pick itself rather
	// than only acted on.
	Probability float64
}

// Gate names, before the pick, questions no operation of the shortlist
// (nor the pick's own three built-ins) can satisfy - a resource the
// catalogue lacks, a verb it has no operation for, a topic outside the
// business domain (docs/specs/midsizing.md's "impossible half": 集計 /
// 承認 / 印刷 on a resource that only lists, is the shape this exists
// for). It is the one typed yes/no decision the local pick is worst at
// (docs/measurements/jev-picker-v1.md, v2.md: 8-11 of the shortlist
// corpus's own losses are the local/jev pick answering `none` on a
// question that was, in fact, answerable), asked separately from the
// pick itself so a wrong yes/no here never contaminates which operation
// gets chosen when the question is answerable.
//
// Orchestrator.planStaged calls Gate after idAffinity narrows the
// shortlist and before o.picker.Pick - see planStaged's own doc comment
// for why a Gate error must never fail the request (fail open: proceed
// to the pick as if no gate were configured at all).
//
// Capability questions ("何ができる？") are not impossible - a Gate
// implementation must answer Impossible: false for them, the same way
// the pick itself routes them to PickListCapabilities rather than
// PickNone (docs/specs/midsizing.md's own capability rows, and make
// eval's "capability"/"real-capability-inventory" cases, both check
// this).
type Gate interface {
	Gate(ctx context.Context, query string, answers []Answer, shortlist domain.Catalog) (GateVerdict, error)
}
