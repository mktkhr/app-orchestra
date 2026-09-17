// Package hybrid composes internal/adapter/planner/jev and
// internal/adapter/planner/pick into one usecase.Picker: try Jev first,
// fall back to the local picker. It holds neither picker's own logic -
// only the decision of which answer to trust - so it imports neither
// package's own internals, just the usecase.Picker interface each one
// already satisfies (constructed elsewhere, with jev.New/pick.New, and
// handed to New below).
//
// The reasoning this composes away: docs/measurements/jev-thresholds.md
// found Jev's own judged confidence tracks its accuracy well above
// roughly 0.7 (v2's [0.7,0.8) band already clears 70%, [0.8,1.0]
// consistently at or above 90%) but is not calibrated below it - the
// [0.4,0.5) band in that same measurement scored only 18.2% correct,
// worse than several lower bands. Jev is also the fast, cheap path
// (docs/measurements/jev-field-report.md notes vendor and practitioner
// reports of large per-call latency wins over sequential calls), so
// paying for its answer only pays off when its own confidence says the
// answer is trustworthy. Below that, or when Jev is slow, erroring, or
// answering with a built-in rather than a catalogue operation, this
// picker falls back to the local picker (internal/adapter/planner/pick)
// instead of trusting a low-confidence guess.
package hybrid

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// defaultJevTimeout bounds how long Pick waits for the Jev half before
// falling back to the local picker, used when New is given no
// WithJevTimeout - short enough that a slow or hanging Jev call never
// makes a hybrid-picked question slower than the local picker alone
// would have been by more than this much.
const defaultJevTimeout = 800 * time.Millisecond

// defaultThreshold is the confidence below which Pick treats a Jev
// PickOperation answer as untrustworthy and falls back to the local
// picker, used when New is given no WithThreshold - the same number
// jev.Gate's own defaultGateThreshold and
// internal/infra/config.defaultJevGateThreshold use, picked for the same
// reason: docs/measurements/jev-thresholds.md's calibration data starts
// looking trustworthy (>=70% correct) at and above this band.
const defaultThreshold = 0.7

// pickFallbackReasonError, pickFallbackReasonTimeout,
// pickFallbackReasonBuiltin and pickFallbackReasonLowConfidence are the
// four non-empty values Pick's own "pick completed" log line's
// pick_fallback_reason carries - empty means Jev's own answer was used,
// no fallback happened.
const (
	pickFallbackReasonError         = "error"
	pickFallbackReasonTimeout       = "timeout"
	pickFallbackReasonBuiltin       = "builtin"
	pickFallbackReasonLowConfidence = "low_confidence"
)

// Option configures a Picker built by New, following the same
// functional-option shape internal/adapter/planner/jev.Option already
// uses.
type Option func(*Picker)

// WithJevTimeout overrides the duration Pick waits for the Jev half
// before falling back to the local picker. The default, defaultJevTimeout,
// is used when this option is never given, or when duration is zero or
// negative.
func WithJevTimeout(duration time.Duration) Option {
	return func(p *Picker) {
		if duration > 0 {
			p.jevTimeout = duration
		}
	}
}

// WithThreshold overrides the confidence below which a Jev PickOperation
// answer is treated as untrustworthy. The default, defaultThreshold, is
// used when this option is never given.
func WithThreshold(threshold float64) Option {
	return func(p *Picker) { p.threshold = threshold }
}

// Picker implements usecase.Picker by trying jevPicker first, under
// jevTimeout, and falling back to localPicker whenever Jev's own answer
// is not trustworthy enough to act on directly - see the package doc
// comment for why. Both fields are usecase.Picker, not the concrete
// *jev.Picker/*pick.Picker types, so this package depends on neither
// adapter directly: the caller (pkg/app.newPicker) is what actually
// builds a jev.Picker and a pick.Picker and hands them here.
type Picker struct {
	jevPicker   usecase.Picker
	localPicker usecase.Picker
	jevTimeout  time.Duration
	threshold   float64
}

var _ usecase.Picker = (*Picker)(nil)

// New builds a Picker trying jevPicker before falling back to
// localPicker, per the fallback rules in Pick's own doc comment.
func New(jevPicker, localPicker usecase.Picker, opts ...Option) *Picker {
	p := &Picker{
		jevPicker:   jevPicker,
		localPicker: localPicker,
		jevTimeout:  defaultJevTimeout,
		threshold:   defaultThreshold,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Pick calls jevPicker.Pick under a context.WithTimeout(ctx, p.jevTimeout)
// derived from ctx, and falls back to localPicker.Pick - called with the
// original, undecorated ctx, so a slow Jev call never eats into the
// local picker's own budget - whenever:
//
//   - jevPicker.Pick returns an error that is not a timeout of the
//     derived context (pick_fallback_reason "error");
//   - jevPicker.Pick returns an error that is, or the derived context
//     itself expired with, context.DeadlineExceeded (pick_fallback_reason
//     "timeout") - checked both ways, since a slow fake or real HTTP
//     client may surface the same deadline either as the returned error
//     or only as the derived context's own Err();
//   - jevPicker.Pick succeeds but answers a built-in (Kind other than
//     usecase.PickOperation: none, list_capabilities or propose_panel)
//     (pick_fallback_reason "builtin");
//   - jevPicker.Pick succeeds with Kind usecase.PickOperation but
//     Confidence below p.threshold (pick_fallback_reason
//     "low_confidence").
//
// Otherwise Jev's own answer is used directly, with an empty
// pick_fallback_reason.
//
// Errors never fail the request (fail open): when localPicker.Pick is
// called and itself also errors, that error is logged at warn and Pick
// returns usecase.Pick{Kind: usecase.PickNone}, nil rather than
// propagating it - there is no error path out of this function at all.
//
// One "pick completed" line is logged per call, on top of - not instead
// of - whichever inner picker's own "pick completed" line already fired
// from within its own Pick call: grepping a platform log for "pick
// completed" sees up to three lines per hybrid decision (Jev's own,
// possibly the local picker's own, and this one), which is expected.
func (p *Picker) Pick(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog,
	planCtx usecase.PlanContext,
) (usecase.Pick, error) {
	start := time.Now()

	jevCtx, cancel := context.WithTimeout(ctx, p.jevTimeout)
	defer cancel()

	jevStart := time.Now()
	jevResult, jevErr := p.jevPicker.Pick(jevCtx, query, answers, turns, shortlist, planCtx)
	jevMS := time.Since(jevStart).Milliseconds()

	reason := fallbackReason(jevCtx, jevResult, jevErr, p.threshold)
	if reason == "" {
		p.logCompleted(ctx, "jev", "", jevResult.Confidence, jevMS, 0)

		return jevResult, nil
	}

	localStart := time.Now()
	localResult, localErr := p.localPicker.Pick(ctx, query, answers, turns, shortlist, planCtx)
	localMS := time.Since(localStart).Milliseconds()

	if localErr != nil {
		slog.Default().WarnContext(ctx, "hybrid pick: local fallback also failed",
			slog.String("pick_fallback_reason", reason), slog.Any("error", localErr),
			slog.Int64("pick_total_ms", time.Since(start).Milliseconds()))

		p.logCompleted(ctx, "local", reason, 0, jevMS, localMS)

		return usecase.Pick{Kind: usecase.PickNone}, nil
	}

	p.logCompleted(ctx, "local", reason, localResult.Confidence, jevMS, localMS)

	return localResult, nil
}

// fallbackReason decides which, if any, of the four fallback reasons
// applies to jevResult/jevErr - see Pick's own doc comment for what each
// one means. "" means Jev's own answer should be used directly.
func fallbackReason(jevCtx context.Context, jevResult usecase.Pick, jevErr error, threshold float64) string {
	if jevErr != nil {
		if errors.Is(jevErr, context.DeadlineExceeded) || errors.Is(jevCtx.Err(), context.DeadlineExceeded) {
			return pickFallbackReasonTimeout
		}

		return pickFallbackReasonError
	}

	if errors.Is(jevCtx.Err(), context.DeadlineExceeded) {
		return pickFallbackReasonTimeout
	}

	if jevResult.Kind != usecase.PickOperation {
		return pickFallbackReasonBuiltin
	}

	if jevResult.Confidence < threshold {
		return pickFallbackReasonLowConfidence
	}

	return ""
}

// logCompleted logs hybrid's own "pick completed" line - see Pick's own
// doc comment for why this is on top of, not instead of, either inner
// picker's own line of the same name.
func (p *Picker) logCompleted(ctx context.Context, provider, reason string, confidence float64, jevMS, localMS int64) {
	slog.Default().InfoContext(ctx, "pick completed",
		slog.String("pick_provider", provider),
		slog.String("pick_fallback_reason", reason),
		slog.Float64("pick_confidence", confidence),
		slog.Int64("pick_jev_ms", jevMS),
		slog.Int64("pick_local_ms", localMS))
}
