package jev_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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

// noulResponseBody builds a fixed "noul" answer body for noul - formatted
// with strconv.FormatFloat rather than encoding/json (there is no error
// path worth a test helper returning one for a handful of fixed literal
// float values this file calls it with).
func noulResponseBody(noul float64) string {
	return `{"model":"jev-1.13.0","answers":{"gate":{"type":"noul","noul":` +
		strconv.FormatFloat(noul, 'f', -1, 64) + `}},"usage":{"input_tokens":11,"output_tokens":2}}`
}

// TestGateSendsStateAsAnObjectWithQuestionAndOperations is this round's
// own "request body exact": the shortlist's two endpoints appear as
// state.operations, in shortlist order, and state.question carries the
// query-plus-answer-lines text stateFor also builds for Picker.
func TestGateSendsStateAsAnObjectWithQuestionAndOperations(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(noulResponseBody(0.1))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	g := jev.NewGate(server.URL, "test-key", nil)

	_, err := g.Gate(context.Background(), "在庫を集計したい", []usecase.Answer{{Param: "id", Value: "42"}}, shortlistCatalog())
	require.NoError(t, err)

	want := map[string]any{
		"state": map[string]any{
			"question": "在庫を集計したい\n回答: id=42",
			"operations": []any{
				map[string]any{"id": "listInventoryItems", "service": "inventory", "summary": "在庫の一覧を返す"},
				map[string]any{"id": "listAttendanceRecords", "service": "attendance", "summary": "勤怠記録の一覧を返す"},
			},
		},
		"model": "jev-latest",
		"questions": map[string]any{
			"gate": map[string]any{
				"type": "noul",
				"instructions": "この質問は、列挙された操作のどれでも実現できないことを求めているか" +
					"（例: 一覧しかない資源の集計・承認・印刷、列挙に無い資源、業務と無関係な話題）。" +
					"能力を尋ねる質問（何ができる？）は「いいえ」。",
			},
		},
	}

	assert.Equal(t, want, gotBody)
}

// TestGateSendsNoAnswerLinesWhenAnswersIsEmpty proves state.question is
// query alone (no trailing "\n") when no answers are given - the same
// no-answers case TestPickSendsStateInstructionsAndCriteria covers for
// Picker.
func TestGateSendsNoAnswerLinesWhenAnswersIsEmpty(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(noulResponseBody(0.1))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	g := jev.NewGate(server.URL, "test-key", nil)

	_, err := g.Gate(context.Background(), "在庫を集計したい", nil, shortlistCatalog())
	require.NoError(t, err)

	state, ok := gotBody["state"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "在庫を集計したい", state["question"])
}

// TestGateNoulAtOrAboveThresholdIsImpossible proves the default
// threshold's ">=" boundary: noul equal to 0.7 is Impossible.
func TestGateNoulAtOrAboveThresholdIsImpossible(t *testing.T) {
	server := stubServer(t, http.StatusOK, noulResponseBody(0.7))
	g := jev.NewGate(server.URL, "test-key", nil)

	verdict, err := g.Gate(context.Background(), "q", nil, shortlistCatalog())
	require.NoError(t, err)
	assert.True(t, verdict.Impossible)
	assert.InDelta(t, 0.7, verdict.Probability, 0.0001)
}

// TestGateNoulBelowThresholdIsPossible proves the same boundary from the
// other side: noul just under 0.7 is not Impossible.
func TestGateNoulBelowThresholdIsPossible(t *testing.T) {
	server := stubServer(t, http.StatusOK, noulResponseBody(0.69))
	g := jev.NewGate(server.URL, "test-key", nil)

	verdict, err := g.Gate(context.Background(), "q", nil, shortlistCatalog())
	require.NoError(t, err)
	assert.False(t, verdict.Impossible)
}

// TestGateWithGateThresholdOverridesTheDefault proves WithGateThreshold
// changes the boundary - a noul that would pass at the default 0.7
// becomes Impossible under a lower threshold.
func TestGateWithGateThresholdOverridesTheDefault(t *testing.T) {
	server := stubServer(t, http.StatusOK, noulResponseBody(0.5))
	g := jev.NewGate(server.URL, "test-key", nil, jev.WithGateThreshold(0.4))

	verdict, err := g.Gate(context.Background(), "q", nil, shortlistCatalog())
	require.NoError(t, err)
	assert.True(t, verdict.Impossible)
}

// TestGateEmptyShortlistIsNotImpossibleWithoutACall mirrors
// TestPickEmptyShortlistIsPickNoneWithoutACall: nothing to gate against,
// so Gate never calls the API at all.
func TestGateEmptyShortlistIsNotImpossibleWithoutACall(t *testing.T) {
	called := false

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(server.Close)

	g := jev.NewGate(server.URL, "test-key", nil)

	verdict, err := g.Gate(context.Background(), "q", nil, domain.Catalog{})
	require.NoError(t, err)
	assert.False(t, verdict.Impossible)
	assert.False(t, called)
}

// TestGateNonTwoHundredIsAnError proves a failing response reaches the
// caller as an error - Gate never fails open on its own; that is
// planStaged's own job (internal/usecase/orchestrator_staging.go).
func TestGateNonTwoHundredIsAnError(t *testing.T) {
	server := stubServer(t, http.StatusInternalServerError, `{"error":"boom"}`)
	g := jev.NewGate(server.URL, "test-key", nil)

	_, err := g.Gate(context.Background(), "q", nil, shortlistCatalog())
	require.Error(t, err)
}

// TestGateMissingAnswerKeyIsAnError proves a 200 response that never
// names the "gate" answer key still surfaces as an error, rather than a
// silently zero-valued verdict.
func TestGateMissingAnswerKeyIsAnError(t *testing.T) {
	server := stubServer(t, http.StatusOK, `{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`)
	g := jev.NewGate(server.URL, "test-key", nil)

	_, err := g.Gate(context.Background(), "q", nil, shortlistCatalog())
	require.Error(t, err)
}

// TestGateCompletedLogCarriesNoulMsAndTokens mirrors
// TestPickCompletedLogCarriesConfidenceTokensAndProvider: the "gate
// completed" line carries every field docs/measurements/jev-picker-v3.md
// asks this round to report from.
func TestGateCompletedLogCarriesNoulMsAndTokens(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	server := stubServer(t, http.StatusOK, noulResponseBody(0.83))
	g := jev.NewGate(server.URL, "test-key", nil)

	_, err := g.Gate(context.Background(), "q", nil, shortlistCatalog())
	require.NoError(t, err)

	logged := buf.String()
	assert.Contains(t, logged, `"msg":"gate completed"`)
	assert.Contains(t, logged, `"gate_noul":0.83`)
	assert.Contains(t, logged, `"gate_ms":`)
	assert.Contains(t, logged, `"gate_input_tokens":11`)
	assert.Contains(t, logged, `"gate_output_tokens":2`)
}
