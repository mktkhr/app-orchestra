package jev_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// statusEnumEndpoint is one safe operation with one enum-valued query
// parameter, "status" - the shape arm 2 (ORCHESTRA_FILL_ENUM=jev) is built
// for.
func statusEnumEndpoint() domain.Endpoint {
	return domain.Endpoint{
		Service:     "inventory",
		OperationID: "listInventoryItems",
		Method:      domain.MethodGet,
		Path:        "/items",
		Parameters: []domain.Parameter{
			{
				Name: "status", In: "query",
				Schema: domain.Schema{
					Type:       domain.SchemaTypeString,
					Enum:       []string{"in_stock", "quarantined"},
					EnumLabels: map[string]string{"in_stock": "入荷済み", "quarantined": "検品保留"},
					Title:      "ステータス",
				},
			},
		},
	}
}

// fillResponseBody builds a fixed wireResponse JSON body for "status"
// alone, the one question statusEnumEndpoint's single parameter produces -
// hand-built, the same reason gate_test.go's own noulResponseBody is,
// rather than json.Marshal over a handful of fixed literal values this
// file calls it with.
func fillResponseBody(choice string, confidence float64) string {
	c := strconv.FormatFloat(confidence, 'f', -1, 64)

	return `{"model":"jev-latest","answers":{"status":{"type":"choice","choice":"` + choice +
		`","confidence":` + c + `,"probabilities":{"` + choice + `":` + c +
		`}}},"usage":{"input_tokens":20,"output_tokens":3}}`
}

// fakeRequest is fakeRequestServer's own result: server is the running
// fixture; gotBody decodes the last request it received - a named result
// pair, not two bare returns, so gocritic's unnamedResult
// (harness/quality/go/golangci.yml) has nothing to flag and a caller reads
// .server/.gotBody by name rather than by position.
type fakeRequest struct {
	server  *httptest.Server
	gotBody func() map[string]any
}

// fakeRequestServer starts an httptest.Server answering every request with
// body, and returns a getter for the last request's own decoded JSON -
// captured through a closure, not a pointer-to-map parameter, so a caller
// never has to hand this function a pointer to a reference type.
func fakeRequestServer(t *testing.T, body string) fakeRequest {
	t.Helper()

	var lastBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		lastBody = raw

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return fakeRequest{server: server, gotBody: func() map[string]any {
		var decoded map[string]any
		if err := json.Unmarshal(lastBody, &decoded); err != nil {
			t.Errorf("decoding captured request body: %v", err)
		}

		return decoded
	}}
}

// fakeServer starts an httptest.Server answering every request with body,
// for a test that never inspects the request itself.
func fakeServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	return fakeRequestServer(t, body).server
}

// TestFillerRequestShapeOneQuestionPerParameterWithBothSentinels is the
// request shape golden test: one Choice question keyed by the parameter's
// own name, its criteria holding every enum value (described by its own
// x-enum-labels label) plus both sentinels.
func TestFillerRequestShapeOneQuestionPerParameterWithBothSentinels(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	gotBody := fake.gotBody()

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok, "questions must be an object")
	require.Len(t, questions, 1, "one question per parameter")

	status, ok := questions["status"].(map[string]any)
	require.True(t, ok, "the one question must be keyed by the parameter's own name")
	assert.Equal(t, "choice", status["type"])

	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "入荷済み", criteria["in_stock"], "enum value described by its own x-enum-labels label")
	assert.Equal(t, "検品保留", criteria["quarantined"])
	assert.Equal(t, "この質問はこの項目を絞り込んでいない", criteria["__unset__"])
	assert.Equal(t, "質問はこの項目を絞り込んでいるが、候補の値のどれとも一致しない", criteria["__mismatch__"])
	assert.Len(t, criteria, 4, "two enum values plus both sentinels, nothing else")

	assert.Equal(t, "jev-latest", gotBody["model"])
	assert.Contains(t, gotBody["state"], "在庫を見せて", "state carries the question text")
}

// TestFillerRequestStateCarriesTurns proves state is built from the same
// helper the local fill's own toolcall.Planner renders turns with -
// TestFillerRequestShapeOneQuestionPerParameterWithBothSentinels already
// covers the query alone; this covers a prior turn appearing in state too.
func TestFillerRequestStateCarriesTurns(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	turns := []usecase.Turn{{Question: "在庫は？", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "listInventoryItems"}}

	_, _, err := f.Fill(context.Background(), &endpoint, "検品中のは？", nil, turns, usecase.PlanContext{})
	require.NoError(t, err)

	state, ok := fake.gotBody()["state"].(string)
	require.True(t, ok)
	assert.Contains(t, state, "在庫は？", "the prior turn's own question text")
	assert.Contains(t, state, "検品中のは？")
}

// TestFillerEveryParameterUnsetIsACallWithNoArguments is the "answers
// map" test: every parameter answering __unset__ becomes a DecisionCall
// with no arguments, the same outcome arm 1 reaches directly.
func TestFillerEveryParameterUnsetIsACallWithNoArguments(t *testing.T) {
	server := fakeServer(t, fillResponseBody("__unset__", 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "listInventoryItems", decision.OperationID)
	assert.Nil(t, decision.Args, "no answered parameter means no arguments at all")
}

// TestFillerAMatchedValueBecomesThatArgument is the "answers map" test's
// other half: a parameter answering one of its own enum values becomes
// that argument on the DecisionCall.
func TestFillerAMatchedValueBecomesThatArgument(t *testing.T) {
	server := fakeServer(t, fillResponseBody("quarantined", 0.95))

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "検品保留のを見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, map[string]any{"status": "quarantined"}, decision.Args)
}

// TestFillerAMismatchBecomesAskOverThatParameter is the mismatch mapping
// test: a __mismatch__ answer becomes a DecisionAsk over that same
// parameter, worded the same way usecase.askForEnumGuess's own question is
// ("<title>はどれですか？") so Orchestrator.ask produces the identical
// ResultKindAsk shape either way.
func TestFillerAMismatchBecomesAskOverThatParameter(t *testing.T) {
	server := fakeServer(t, fillResponseBody("__mismatch__", 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "だめなやつを見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionAsk, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "listInventoryItems", decision.OperationID)
	assert.Equal(t, "status", decision.Param)
	assert.Equal(t, "ステータスはどれですか？", decision.Question)
}

// TestFillerFailsOpenOnLowConfidence proves an answer's confidence below
// the configured threshold fails open: ok is false, err is nil - never an
// error the caller must act on.
func TestFillerFailsOpenOnLowConfidence(t *testing.T) {
	server := fakeServer(t, fillResponseBody("in_stock", 0.4))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillThreshold(0.5))
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, usecase.Decision{}, decision)
}

// TestFillerFailsOpenOnMissingAnswer proves a response naming no answer at
// all for a parameter's own question fails open the same way a
// low-confidence one does.
func TestFillerFailsOpenOnMissingAnswer(t *testing.T) {
	body := `{"model":"jev-latest","answers":{},"usage":{"input_tokens":5,"output_tokens":1}}`
	server := fakeServer(t, body)

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestFillerFailsOpenOnTransportError proves a non-2xx response fails
// open too, this time with a non-nil error the caller logs but never
// surfaces to the person asking.
func TestFillerFailsOpenOnTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.Error(t, err)
	assert.False(t, ok)
}
