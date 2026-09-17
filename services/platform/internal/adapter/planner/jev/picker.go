package jev

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// defaultAmbiguityThreshold is the confidence below which Pick judges an
// answer Ambiguous (S4) when the caller passes no WithAmbiguityThreshold.
const defaultAmbiguityThreshold = 0.5

// CriteriaV1 and CriteriaV2 are WithCriteria's two accepted values.
// CriteriaV1 is criteriaFor's original one-line-per-option shape,
// unchanged since this adapter's first trial. CriteriaV2 is the richer
// per-option object (criteriaForV2: `what`, `examples`, `not_for`) the
// Jev trial's second round measures against it. An empty string (the
// zero value app.Picker.JevCriteria has when a caller builds one without
// setting it - every test and caller that predates CriteriaV2) is
// treated exactly as CriteriaV1, so New's behavior is unchanged for
// anyone who never mentions this option at all.
const (
	CriteriaV1 = "v1"
	CriteriaV2 = "v2"
)

// Option configures a Picker built by New.
type Option func(*Picker)

// WithAmbiguityThreshold overrides the confidence below which Pick's
// answer is judged Ambiguous. The default, defaultAmbiguityThreshold, is
// used when this option is never given.
func WithAmbiguityThreshold(threshold float64) Option {
	return func(p *Picker) { p.ambiguityThreshold = threshold }
}

// WithCriteria selects which of criteriaFor (CriteriaV1) or
// criteriaForV2 (CriteriaV2) buildRequest uses to build each shortlist
// entry's criteria. "" is treated as CriteriaV1 (see CriteriaV1's own
// doc comment); any other value is also folded to CriteriaV1 rather than
// panicking or silently sending no criteria at all - a typo'd
// ORCHESTRA_JEV_CRITERIA already fails startup in
// internal/infra/config.parseJevCriteria, so this fallback only ever
// matters for a caller outside that path, such as a test.
func WithCriteria(criteria string) Option {
	return func(p *Picker) {
		if criteria == CriteriaV2 {
			p.criteria = CriteriaV2

			return
		}

		p.criteria = CriteriaV1
	}
}

// WithFanOutGate turns on the v5 trial's own fan-out
// (docs/measurements/jev-picker-v5.md, "fan-out, not extra calls"): every
// Pick call adds the "impossible" noul question (addImpossibleQuestion)
// to the same request as "pick", and Pick itself judges Impossible - at or
// above threshold - and short-circuits to usecase.Pick{Kind: PickNone}
// without ever examining the pick answer, the same outcome
// planStaged reaches today by calling a standalone usecase.Gate first
// (orchestrator_staging.go) - but in one HTTP call, not two. threshold <=
// 0 is treated as "keep the default" (defaultGateThreshold, the same
// number jev.NewGate's own zero-threshold convention uses,
// pkg/app.newGate's doc comment) rather than a threshold no answer could
// ever clear.
//
// This is Picker's own alternative to pairing it with a standalone Gate
// (WithGate, orchestrator.go): pkg/app.stagingOptions builds a Picker with
// this option, and skips usecase.WithGate entirely, exactly when both
// Picker.Name and Gate.Name are "jev" - GateJev paired with the local
// picker still goes through jev.Gate, the standalone port, since there is
// no single request to fold it into there.
func WithFanOutGate(threshold float64) Option {
	return func(p *Picker) {
		p.fanOutGate = true

		if threshold > 0 {
			p.gateThreshold = threshold
		}
	}
}

// WithLegacyInstructions makes Pick send legacyInstructionsFor's plain
// string as the "pick" question's own instructions, instead of
// instructionsFor's v5 object - a temporary isolation switch
// (docs/measurements/jev-v5.md, run B): turns still reach "state"
// unchanged, so a run built with this option tells the v5 round's object
// instructions apart from its turns-in-state change as the cause of
// real-attendance-detail's new regression. ORCHESTRA_JEV_LEGACY_INSTRUCTIONS
// (internal/infra/config) is this option's own env-var switch.
func WithLegacyInstructions() Option {
	return func(p *Picker) { p.legacyInstructions = true }
}

// Picker implements usecase.Picker over TypeSafe's Jev API: the shortlist
// and the three fixed built-ins (see criteriaFor) as one "choice"
// question, with Jev's own judged confidence read back as S4's Ambiguous
// - a call this adapter takes from Jev directly, rather than the local
// picker's own "ambiguous" text token (internal/adapter/planner/pick's
// ambiguousPattern).
type Picker struct {
	client             *client
	ambiguityThreshold float64
	// criteria is CriteriaV1 or CriteriaV2, set by WithCriteria; the zero
	// value ("") is treated as CriteriaV1 by buildRequest.
	criteria string
	// fanOutGate and gateThreshold are WithFanOutGate's own fields: fold
	// the "impossible" noul question into every Pick request, and judge it
	// Impossible at or above gateThreshold.
	fanOutGate    bool
	gateThreshold float64
	// legacyInstructions is WithLegacyInstructions' own field.
	legacyInstructions bool
}

var _ usecase.Picker = (*Picker)(nil)

// New builds a Picker calling baseURL with apiKey. A nil httpClient
// defaults to http.DefaultClient.
func New(baseURL, apiKey string, httpClient *http.Client, opts ...Option) *Picker {
	p := &Picker{
		client:             newClient(baseURL, apiKey, httpClient),
		ambiguityThreshold: defaultAmbiguityThreshold,
		gateThreshold:      defaultGateThreshold,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Pick sends query, answers, turns and shortlist to Jev as one "choice"
// question (buildRequest) - plus, under WithFanOutGate, the "impossible"
// noul question in the same request (addImpossibleQuestion) - and maps
// the "pick" answer back into a usecase.Pick (mapAnswer). An empty
// shortlist is PickNone without calling the API at all - the same
// short-circuit internal/adapter/planner/pick.Picker takes - and the call
// is bounded to requestTimeout regardless of ctx's own deadline.
func (p *Picker) Pick(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog,
) (usecase.Pick, error) {
	if len(shortlist.Endpoints) == 0 {
		return usecase.Pick{Kind: usecase.PickNone}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	start := time.Now()

	req := buildRequest(query, answers, turns, shortlist, p.criteria, p.legacyInstructions)
	if p.fanOutGate {
		addImpossibleQuestion(&req)
	}

	resp, err := p.client.pick(ctx, req)
	if err != nil {
		return usecase.Pick{}, fmt.Errorf("jev picking: %w", err)
	}

	if p.fanOutGate {
		if impossible, ok := resp.Answers[impossibleQuestionName]; ok {
			gateImpossible := impossible.Noul >= p.gateThreshold

			slog.Default().InfoContext(ctx, "gate completed",
				slog.Bool("gate_fanout", true),
				slog.Float64("gate_noul", impossible.Noul),
				slog.Bool("gate_impossible", gateImpossible),
				slog.Int64("gate_ms", time.Since(start).Milliseconds()))

			if gateImpossible {
				return usecase.Pick{Kind: usecase.PickNone}, nil
			}
		}
	}

	answer, ok := resp.Answers[questionName]
	if !ok {
		return usecase.Pick{}, fmt.Errorf("%w: response named no %q answer", ErrRequestFailed, questionName)
	}

	result, matched := mapAnswer(answer, shortlist, p.ambiguityThreshold)
	if !matched {
		slog.Default().WarnContext(ctx, "jev pick named an unknown choice", slog.String("choice", answer.Choice))
	}

	slog.Default().InfoContext(ctx, "pick completed",
		slog.String("pick_operation_id", result.OperationID),
		// pick_choice is Jev's own raw answer - unlike pick_operation_id,
		// never empty, so a log line for a built-in (list_capabilities,
		// propose_panel, none) still names what was chosen, not just
		// what usecase.Pick.OperationID leaves blank for those kinds.
		slog.String("pick_choice", answer.Choice),
		slog.Bool("pick_ambiguous", result.Ambiguous),
		slog.Int64("pick_ms", time.Since(start).Milliseconds()),
		slog.Float64("pick_confidence", answer.Confidence),
		slog.Int("pick_input_tokens", resp.Usage.InputTokens),
		slog.Int("pick_output_tokens", resp.Usage.OutputTokens),
		slog.String("pick_provider", "jev"),
		// pick_probabilities is Jev's own per-candidate probability
		// distribution for this "pick" answer - added 2026-09-17 for the
		// Jev trial measurement (docs/specs/staging.md S4's confidence
		// work), so a run's platform log alone is enough to reconstruct
		// each pick's full distribution without a second, debug-only
		// line to correlate against.
		slog.Any("pick_probabilities", answer.Probabilities))

	return result, nil
}
