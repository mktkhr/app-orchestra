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

// unsetChoice, mismatchChoice, refusalChoice and capabilitiesChoice are the
// sentinel options every question Filler.Fill asks carries beside its
// parameter's own enum values (docs/measurements/jev-conditions.md, arm 2):
// unsetChoice means the question never restricted this parameter at all;
// mismatchChoice means it did, but to a value none of the enum's own
// options match - the same situation usecase's own enum-guess guard
// (orchestrator_enum_guess.go) and askDegrade exist for on the local
// fill's side. refusalChoice and capabilitiesChoice are the same two-way
// split the local fill's own decline has had since 2026-09-16
// (DecisionNone/DecisionListCapabilities): refusalChoice means the picked
// operation cannot answer the question at all - its subject or kind does
// not match what the question asks for, not merely one parameter's value;
// capabilitiesChoice means the question is not about running any
// operation at all, but about what the system can do in general - a
// request the plain enum classifier above has no way to exercise, and
// which is not the same claim as refusalChoice's (a wrong operation for
// this question) even though both end the pick without calling it. Both are
// only ever criteria of a parameter's own Choice question in
// refusalModeInOptions (ORCHESTRA_FILL_ENUM_REFUSAL=1); in
// refusalModeSeparate their own Japanese wording (refusalLabel,
// capabilitiesLabel) is reused as two whole-request "noul" questions
// instead (addSeparateRefusalQuestions, filler_mapping.go) - see that
// function's own doc comment for why a claim about the whole request does
// not belong among one field's own options.
const (
	unsetChoice        = "__unset__"
	mismatchChoice     = "__mismatch__"
	refusalChoice      = "__refusal__"
	capabilitiesChoice = "__capabilities__"
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
	unsetLabelWide = "この質問はこの項目を絞り込んでいない、またはこの項目についてすべてを対象にするよう明示している（今の会話の続きで絞り込みを解除した場合を含む）"
	mismatchLabel  = "質問はこの項目を絞り込んでいるが、候補の値のどれとも一致しない"
	refusalLabel   = "選ばれた操作は、そもそもこの質問には答えられない（操作の対象や種類が質問と合っていない）"
	// capabilitiesLabel is capabilitiesChoice's criterion text: the
	// person is not asking to run any operation at all, but asking what
	// the system can do or which operations exist in general - worded
	// generically (no eval question, no service named) and kept
	// distinct from refusalLabel's own wording, which is about this one
	// operation being wrong for the question, not about the question
	// being a capabilities question at all.
	capabilitiesLabel = "質問は特定の操作の実行を求めているのではなく、そもそもこの仕組み全体で何ができるか、どんな操作があるかを尋ねている"
	askQuestionText   = "はどれですか？"
)

// sentinelCount is how many sentinel options (unsetChoice, mismatchChoice,
// refusalChoice, capabilitiesChoice) criteriaForParam adds beside a
// parameter's own enum values, at most - named so the map capacity hint
// isn't a bare magic number (mnd, harness/quality/go/golangci.yml).
// refusalChoice and capabilitiesChoice are only added in refusalModeInOptions
// (WithFillRefusal), so this is a capacity hint, not always the exact count -
// refusalModeSeparate (WithFillRefusalSeparate) never adds either to a
// parameter's own criteria at all (they become their own whole-request
// questions instead, addSeparateRefusalQuestions in filler_mapping.go).
const sentinelCount = 4

// refusalModeInOptions and refusalModeSeparate are Filler.refusalMode's two
// non-default values, mirroring ORCHESTRA_FILL_ENUM_REFUSAL's own "1" and
// "separate" (internal/infra/config.FillEnumRefusalOn/Separate). The zero
// value ("") means refusalMode is off: neither sentinel, nor either
// whole-request question, is ever sent.
const (
	refusalModeInOptions = "1"
	refusalModeSeparate  = "separate"
)

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
	client    *client
	threshold float64
	clock     func() time.Time
	// refusalMode is "" (off), refusalModeInOptions or refusalModeSeparate -
	// set by WithFillRefusal/WithFillRefusalSeparate, mutually exclusive
	// since a Filler is built with at most one of them.
	refusalMode  string
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

// WithFillRefusal turns on ORCHESTRA_FILL_ENUM_REFUSAL=1: refusalChoice and
// capabilitiesChoice are both added to every parameter's Choice question -
// one option set, not two, since a Filler is either built with this or
// without it. An answer that picks refusalChoice on any parameter makes
// Fill return usecase.Decision{Kind: usecase.DecisionNone}; one that picks
// capabilitiesChoice makes it return usecase.Decision{Kind:
// usecase.DecisionListCapabilities} - the same two outcomes the local fill
// produces when it declines a pick outright, not a new result shape
// (resolvePickedFill/planPicked, internal/usecase). Off by default: a
// Filler built without this option (nor WithFillRefusalSeparate) never
// offers either sentinel, nor either whole-request question.
func WithFillRefusal() FillerOption {
	return func(f *Filler) { f.refusalMode = refusalModeInOptions }
}

// WithFillRefusalSeparate turns on ORCHESTRA_FILL_ENUM_REFUSAL=separate: the
// same two whole-request judgements WithFillRefusal offers, but asked as
// their own "noul" questions in the same request (addSeparateRefusalQuestions,
// filler_mapping.go) rather than as criteria on every parameter's own Choice
// question - a claim about the whole request no longer competes inside a
// field's own option list (docs/measurements/jev-conditions.md: two failing
// ListInventoryItems rows where "the question does not restrict status" won
// over the whole-request refusal claim it was never actually up against).
// Mutually exclusive with WithFillRefusal in practice (internal/infra/config
// only ever reads one of "1"/"separate" from ORCHESTRA_FILL_ENUM_REFUSAL);
// whichever option is given last wins if a caller passes both.
func WithFillRefusalSeparate() FillerOption {
	return func(f *Filler) { f.refusalMode = refusalModeSeparate }
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

	outcome, ok := mapFillAnswers(endpoint, resp, f.threshold, f.refusalMode)
	fillMs := time.Since(start).Milliseconds()

	if !ok {
		slog.Default().WarnContext(ctx, "fill_enum answer missing or below threshold, falling back to local fill",
			slog.String("fill_provider", "local"), slog.String("service", endpoint.Service),
			slog.String("operation_id", endpoint.OperationID), slog.Int64("fill_ms", fillMs))

		return usecase.Decision{}, false, nil
	}

	logFillCompleted(ctx, endpoint, &outcome, fillMs, resp.Usage, f.refusalMode)

	return outcome.decision(endpoint), true, nil
}
