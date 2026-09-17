// gate.go: this package's other half, usecase.Gate, over the same POST
// /v1/systemone endpoint picker.go's Picker calls, for the v3 Jev trial
// (v3: a "noul refusal gate" in front of the local pick,
// docs/measurements/jev-picker-v3.md): one "noul" question, asked before
// the pick, judging whether query is answerable at all against the
// shortlist it was given. The package's own doc comment is client.go's.

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

// defaultGateThreshold is the noul probability at or above which Gate
// judges a question Impossible, when the caller passes no
// WithGateThreshold - the v3 trial's own starting point
// (docs/measurements/jev-picker-v3.md), overridable per deployment
// through internal/infra/config's ORCHESTRA_JEV_GATE_THRESHOLD.
const defaultGateThreshold = 0.7

// gateQuestionName is the one key this adapter's gateWireRequest.Questions
// and gateWireResponse.Answers ever use - gate's own counterpart to
// picker.go's questionName ("pick").
const gateQuestionName = "gate"

// gateInstructions is the "noul" question Gate asks: whether query asks
// for something none of shortlist's operations (nor the pick's own three
// built-ins) can do at all - a resource with only a list operation asked
// to be aggregated, approved or printed; a resource outside shortlist
// entirely; a topic outside the business domain. Capability questions
// ("何ができる？") are explicitly carved out as "no" - they are always
// answerable (by PickListCapabilities), never a case for this gate to
// refuse (docs/specs/midsizing.md's own capability rows; make eval's
// "capability"/"real-capability-inventory" cases check this).
const gateInstructions = "この質問は、列挙された操作のどれでも実現できないことを求めているか" +
	"（例: 一覧しかない資源の集計・承認・印刷、列挙に無い資源、業務と無関係な話題）。" +
	"能力を尋ねる質問（何ができる？）は「いいえ」。"

// gateStateOperation is one shortlist endpoint as gateState's own
// "operations" list carries it - a compact stand-in for criteriaFor's
// full criteria map (Picker's own request), since Gate only needs enough
// of each operation to judge whether it could possibly answer query, not
// a criterion rich enough to choose among them.
type gateStateOperation struct {
	ID      string `json:"id"`
	Service string `json:"service"`
	Summary string `json:"summary"`
}

// gateState is the object Gate sends as its systemone request's "state" -
// an object, not the plain string stateFor builds for Picker (per
// docs.typesafe.ai/primitives/noul, state may be any JSON value; an
// object here lets Question and Operations be told apart on the wire
// rather than folded into one string the model has to re-parse itself).
type gateState struct {
	Question   string               `json:"question"`
	Operations []gateStateOperation `json:"operations"`
}

// gateWireRequest is the JSON body Gate's POST /v1/systemone expects -
// gate's own counterpart to mapping.go's wireRequest, differing only in
// State's type (an object, not a string).
type gateWireRequest struct {
	State     gateState                   `json:"state"`
	Model     string                      `json:"model"`
	Questions map[string]gateWireQuestion `json:"questions"`
}

// gateWireQuestion is one entry of gateWireRequest.Questions - always
// "noul", with no criteria (docs.typesafe.ai/primitives/noul's own
// "true"/"false" criteria object is optional and left unset here:
// gateInstructions already states both directions in one sentence).
type gateWireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
}

// gateWireResponse is the JSON body POST /v1/systemone answers Gate's
// request with.
type gateWireResponse struct {
	Model   string                    `json:"model"`
	Answers map[string]gateWireAnswer `json:"answers"`
	Usage   wireUsage                 `json:"usage"`
}

// gateWireAnswer is one entry of gateWireResponse.Answers: a "noul"
// answer's own shape, "noul" being the probability the answer is "yes" -
// here, the probability query is impossible against the shortlist it was
// asked about (docs.typesafe.ai/primitives/noul).
type gateWireAnswer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

// gateOperationsFor builds gateState's own "operations" list, one entry
// per shortlist endpoint, in shortlist's own order - id/service/summary
// only, summaryFor being the same Summary-or-Description-first-line
// fallback criteriaFor's own "what" column uses.
func gateOperationsFor(shortlist domain.Catalog) []gateStateOperation {
	operations := make([]gateStateOperation, len(shortlist.Endpoints))

	for i := range shortlist.Endpoints {
		e := &shortlist.Endpoints[i]
		operations[i] = gateStateOperation{ID: e.OperationID, Service: e.Service, Summary: summaryFor(e)}
	}

	return operations
}

// buildGateRequest builds the one gateWireRequest Gate.Gate sends for
// query, answers and shortlist: stateFor's own query-plus-answer-lines
// text as gateState.Question (byte for byte the same text Picker's own
// "state" string would be, reused rather than re-implemented), shortlist
// as gateState.Operations, and the one fixed "noul" question.
func buildGateRequest(query string, answers []usecase.Answer, shortlist domain.Catalog) gateWireRequest {
	return gateWireRequest{
		State: gateState{Question: stateFor(query, answers), Operations: gateOperationsFor(shortlist)},
		Model: modelName,
		Questions: map[string]gateWireQuestion{
			gateQuestionName: {Type: "noul", Instructions: gateInstructions},
		},
	}
}

// GateOption configures a Gate built by NewGate.
type GateOption func(*Gate)

// WithGateThreshold overrides the noul probability at or above which
// Gate.Gate judges a question Impossible. The default,
// defaultGateThreshold, is used when this option is never given.
func WithGateThreshold(threshold float64) GateOption {
	return func(g *Gate) { g.threshold = threshold }
}

// Gate implements usecase.Gate over TypeSafe's Jev API: one "noul"
// question per call (gateInstructions), judged Impossible when the
// answer's own noul probability is at or above g.threshold
// (docs/measurements/jev-picker-v3.md).
type Gate struct {
	client    *client
	threshold float64
}

var _ usecase.Gate = (*Gate)(nil)

// NewGate builds a Gate calling baseURL with apiKey. A nil httpClient
// defaults to http.DefaultClient - same as New (Picker's own
// constructor); Gate and Picker never share a *client instance (each
// call site builds its own), since neither carries connection state
// beyond what a shared *http.Client would already pool.
func NewGate(baseURL, apiKey string, httpClient *http.Client, opts ...GateOption) *Gate {
	g := &Gate{
		client:    newClient(baseURL, apiKey, httpClient),
		threshold: defaultGateThreshold,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// Gate sends query, answers and shortlist to Jev as one "noul" question
// (buildGateRequest) and reports Impossible when the answer's own noul
// probability is at or above g.threshold. An empty shortlist has nothing
// to gate against - it returns GateVerdict{Impossible: false} without
// calling the API at all, the same short-circuit Picker.Pick takes for an
// empty shortlist (mapped to PickNone there; here, simply "not
// impossible", leaving whatever comes next - which never reaches an empty
// shortlist in practice - to decide). The call is bounded to
// requestTimeout regardless of ctx's own deadline, same as Picker.Pick.
//
// Errors are returned, not swallowed: Orchestrator.planStaged
// (internal/usecase/orchestrator_staging.go) is where a Gate outage fails
// open, by design - this method's job is only to report what happened,
// truthfully, not to decide whether a caller can tolerate it.
func (g *Gate) Gate(
	ctx context.Context, query string, answers []usecase.Answer, shortlist domain.Catalog,
) (usecase.GateVerdict, error) {
	if len(shortlist.Endpoints) == 0 {
		return usecase.GateVerdict{Impossible: false}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	start := time.Now()

	resp, err := g.client.gate(ctx, buildGateRequest(query, answers, shortlist))
	if err != nil {
		return usecase.GateVerdict{}, fmt.Errorf("jev gating: %w", err)
	}

	answer, ok := resp.Answers[gateQuestionName]
	if !ok {
		return usecase.GateVerdict{}, fmt.Errorf("%w: response named no %q answer", ErrRequestFailed, gateQuestionName)
	}

	verdict := usecase.GateVerdict{Impossible: answer.Noul >= g.threshold, Probability: answer.Noul}

	slog.Default().InfoContext(ctx, "gate completed",
		slog.Float64("gate_noul", answer.Noul),
		slog.Int64("gate_ms", time.Since(start).Milliseconds()),
		slog.Int("gate_input_tokens", resp.Usage.InputTokens),
		slog.Int("gate_output_tokens", resp.Usage.OutputTokens))

	return verdict, nil
}
