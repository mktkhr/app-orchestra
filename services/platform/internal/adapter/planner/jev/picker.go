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

// Option configures a Picker built by New.
type Option func(*Picker)

// WithAmbiguityThreshold overrides the confidence below which Pick's
// answer is judged Ambiguous. The default, defaultAmbiguityThreshold, is
// used when this option is never given.
func WithAmbiguityThreshold(threshold float64) Option {
	return func(p *Picker) { p.ambiguityThreshold = threshold }
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
}

var _ usecase.Picker = (*Picker)(nil)

// New builds a Picker calling baseURL with apiKey. A nil httpClient
// defaults to http.DefaultClient.
func New(baseURL, apiKey string, httpClient *http.Client, opts ...Option) *Picker {
	p := &Picker{
		client:             newClient(baseURL, apiKey, httpClient),
		ambiguityThreshold: defaultAmbiguityThreshold,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Pick sends query, answers and shortlist to Jev as one "choice" question
// (buildRequest) and maps its answer back into a usecase.Pick
// (mapAnswer). An empty shortlist is PickNone without calling the API at
// all - the same short-circuit internal/adapter/planner/pick.Picker takes
// - and the call is bounded to requestTimeout regardless of ctx's own
// deadline.
func (p *Picker) Pick(
	ctx context.Context, query string, answers []usecase.Answer, shortlist domain.Catalog,
) (usecase.Pick, error) {
	if len(shortlist.Endpoints) == 0 {
		return usecase.Pick{Kind: usecase.PickNone}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	start := time.Now()

	resp, err := p.client.pick(ctx, buildRequest(query, answers, shortlist))
	if err != nil {
		return usecase.Pick{}, fmt.Errorf("jev picking: %w", err)
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
