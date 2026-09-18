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

// unsetChoice and mismatchChoice are the two sentinel options every
// question Filler.Fill asks carries beside its parameter's own enum values
// (docs/measurements/jev-conditions.md, arm 2): unsetChoice means the
// question never restricted this parameter at all; mismatchChoice means it
// did, but to a value none of the enum's own options match - the same
// situation usecase's own enum-guess guard (orchestrator_enum_guess.go)
// and askDegrade exist for on the local fill's side.
const (
	unsetChoice     = "__unset__"
	mismatchChoice  = "__mismatch__"
	unsetLabel      = "この質問はこの項目を絞り込んでいない"
	mismatchLabel   = "質問はこの項目を絞り込んでいるが、候補の値のどれとも一致しない"
	askQuestionText = "はどれですか？"
)

// sentinelCount is how many sentinel options (unsetChoice, mismatchChoice)
// criteriaForParam adds beside a parameter's own enum values - named so
// the map capacity hint isn't a bare magic number (mnd,
// harness/quality/go/golangci.yml).
const sentinelCount = 2

// defaultFillThreshold is used when WithFillThreshold is never given -
// ORCHESTRA_FILL_ENUM_THRESHOLD's own default (internal/infra/config).
const defaultFillThreshold = 0.5

// Filler implements usecase.Filler over TypeSafe's Jev API, replacing the
// fill's own model call for a picked operation whose every parameter is
// enum-valued (usecase.EligibleForFillEnum): one Choice question per
// parameter, in a single request, rather than the picker's own one
// question for the whole shortlist.
type Filler struct {
	client    *client
	threshold float64
	clock     func() time.Time
}

var _ usecase.Filler = (*Filler)(nil)

// FillerOption configures a Filler built by NewFiller.
type FillerOption func(*Filler)

// WithFillThreshold overrides defaultFillThreshold: any question's answer
// confidence below it makes Fill fail open (see Fill's own doc comment).
func WithFillThreshold(threshold float64) FillerOption {
	return func(f *Filler) { f.threshold = threshold }
}

// WithFillerClock overrides the source of "today" the shared state
// (toolcall.BuildUserContent) is rendered with - time.Now by default, so
// the running platform always sends the real date; a test pins it instead,
// the same reason toolcall.WithClock exists.
func WithFillerClock(clock func() time.Time) FillerOption {
	return func(f *Filler) { f.clock = clock }
}

// NewFiller builds a Filler calling baseURL with apiKey. A nil httpClient
// defaults to http.DefaultClient.
func NewFiller(baseURL, apiKey string, httpClient *http.Client, opts ...FillerOption) *Filler {
	f := &Filler{
		client:    newClient(baseURL, apiKey, httpClient),
		threshold: defaultFillThreshold,
		clock:     time.Now,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// Fill sends endpoint's parameters to Jev as one Choice question per
// parameter (buildFillRequest) and maps the answers back (mapFillAnswers):
// see usecase.Filler's own doc comment for what ok/err mean to the caller.
// A transport error is both logged (warn) and returned; a missing answer
// or a below-threshold confidence is logged (warn) and returned as ok ==
// false with a nil error - never an error the caller must act on.
func (f *Filler) Fill(
	ctx context.Context, endpoint *domain.Endpoint, query string, answers []usecase.Answer, turns []usecase.Turn,
	_ usecase.PlanContext,
) (usecase.Decision, bool, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req := buildFillRequest(endpoint, query, answers, turns, f.clock())

	resp, err := f.client.pick(ctx, req)
	if err != nil {
		fillMs := time.Since(start).Milliseconds()

		slog.Default().WarnContext(ctx, "fill_enum request failed, falling back to local fill",
			slog.String("fill_provider", "local"), slog.String("service", endpoint.Service),
			slog.String("operation_id", endpoint.OperationID), slog.Int64("fill_ms", fillMs), slog.Any("error", err))

		return usecase.Decision{}, false, fmt.Errorf("jev filling: %w", err)
	}

	outcome, ok := mapFillAnswers(endpoint, resp, f.threshold)
	fillMs := time.Since(start).Milliseconds()

	if !ok {
		slog.Default().WarnContext(ctx, "fill_enum answer missing or below threshold, falling back to local fill",
			slog.String("fill_provider", "local"), slog.String("service", endpoint.Service),
			slog.String("operation_id", endpoint.OperationID), slog.Int64("fill_ms", fillMs))

		return usecase.Decision{}, false, nil
	}

	logFillCompleted(ctx, endpoint, outcome, fillMs, resp.Usage)

	return outcome.decision(endpoint), true, nil
}
