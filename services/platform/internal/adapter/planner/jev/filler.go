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

// unsetChoice, mismatchChoice and refusalChoice are the sentinel options
// every question Filler.Fill asks carries beside its parameter's own enum
// values (docs/measurements/jev-conditions.md, arm 2): unsetChoice means
// the question never restricted this parameter at all; mismatchChoice
// means it did, but to a value none of the enum's own options match - the
// same situation usecase's own enum-guess guard
// (orchestrator_enum_guess.go) and askDegrade exist for on the local
// fill's side. refusalChoice (ORCHESTRA_FILL_ENUM_REFUSAL, off by
// default) means the picked operation cannot answer the question at all -
// its subject or kind does not match what the question asks for, not
// merely one parameter's value - the same power the local fill has had to
// decline a pick outright (DecisionNone/DecisionListCapabilities) since
// 2026-09-16, which the plain enum classifier above has no way to
// exercise.
const (
	unsetChoice    = "__unset__"
	mismatchChoice = "__mismatch__"
	refusalChoice  = "__refusal__"
	// unsetLabelNarrow is unsetChoice's criterion text when
	// ORCHESTRA_FILL_ENUM_UNSET_WORDING is unset or "narrow" (the
	// default) - unchanged since arm 2 was introduced.
	unsetLabelNarrow = "この質問はこの項目を絞り込んでいない"
	// unsetLabelWide is unsetChoice's criterion text when
	// ORCHESTRA_FILL_ENUM_UNSET_WORDING is "wide": the same case
	// unsetLabelNarrow covers, plus a question that explicitly asks for
	// everything on this field or removes a restriction an earlier turn
	// in the same conversation placed on it (the d06 turn 2 regression,
	// 「やっぱり全部」read as __mismatch__ rather than __unset__).
	unsetLabelWide  = "この質問はこの項目を絞り込んでいない、またはこの項目についてすべてを対象にするよう明示している（今の会話の続きで絞り込みを解除した場合を含む）"
	mismatchLabel   = "質問はこの項目を絞り込んでいるが、候補の値のどれとも一致しない"
	refusalLabel    = "選ばれた操作は、そもそもこの質問には答えられない（操作の対象や種類が質問と合っていない）"
	askQuestionText = "はどれですか？"
)

// sentinelCount is how many sentinel options (unsetChoice, mismatchChoice,
// refusalChoice) criteriaForParam adds beside a parameter's own enum
// values, at most - named so the map capacity hint isn't a bare magic
// number (mnd, harness/quality/go/golangci.yml). refusalChoice is only
// added when the Filler was built WithFillRefusal, so this is a capacity
// hint, not always the exact count.
const sentinelCount = 3

// unsetWordingWide is the internal marker Filler.unsetWording is set to by
// WithFillUnsetWordingWide - the zero value ("") means unsetLabelNarrow,
// the default.
const unsetWordingWide = "wide"

// defaultFillThreshold is used when WithFillThreshold is never given -
// ORCHESTRA_FILL_ENUM_THRESHOLD's own default (internal/infra/config).
const defaultFillThreshold = 0.5

// Filler implements usecase.Filler over TypeSafe's Jev API, replacing the
// fill's own model call for a picked operation whose every parameter is
// enum-valued (usecase.EligibleForFillEnum): one Choice question per
// parameter, in a single request, rather than the picker's own one
// question for the whole shortlist.
type Filler struct {
	client       *client
	threshold    float64
	clock        func() time.Time
	refusal      bool
	unsetWording string
}

var _ usecase.Filler = (*Filler)(nil)

// FillerOption configures a Filler built by NewFiller.
type FillerOption func(*Filler)

// WithFillThreshold overrides defaultFillThreshold: any question's answer
// confidence below it makes Fill fail open (see Fill's own doc comment).
func WithFillThreshold(threshold float64) FillerOption {
	return func(f *Filler) { f.threshold = threshold }
}

// WithFillRefusal turns on ORCHESTRA_FILL_ENUM_REFUSAL: refusalChoice is
// added to every parameter's Choice question, and an answer that picks it
// on any parameter makes Fill return usecase.Decision{Kind:
// usecase.DecisionNone} - the same outcome the local fill produces when it
// declines a pick outright, not a new result shape
// (resolvePickedFill/planPicked, internal/usecase). Off by default: a
// Filler built without this option never offers refusalChoice at all.
func WithFillRefusal() FillerOption {
	return func(f *Filler) { f.refusal = true }
}

// WithFillUnsetWordingWide turns on ORCHESTRA_FILL_ENUM_UNSET_WORDING=wide:
// unsetChoice's criterion text additionally covers a question that
// explicitly asks for everything on this field or removes an earlier
// turn's restriction (unsetLabelWide's own doc comment). Off by default: a
// Filler built without this option keeps unsetLabelNarrow, byte-identical
// to arm 2's original wording.
func WithFillUnsetWordingWide() FillerOption {
	return func(f *Filler) { f.unsetWording = unsetWordingWide }
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

	req := f.buildFillRequest(endpoint, query, answers, turns, f.clock())

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
