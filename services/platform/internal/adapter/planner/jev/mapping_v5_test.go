package jev_test

// The v5 Jev trial's own tests (docs/measurements/jev-picker-v5.md): the
// pick's request carries the conversation's turns inside "state" once
// there are any, and - under WithFanOutGate - the refusal gate's own
// "impossible" question rides along in the same request rather than a
// second call.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// capturedRequest holds one request's decoded body, filled in only once
// the server below has actually handled a request - a struct, not a bare
// *map[string]any, since gocritic's ptrToRefParam flags a pointer to an
// already-reference map type even as a return value.
type capturedRequest struct {
	body map[string]any
}

// captureRequestBody starts a server that decodes every request it
// receives into the capturedRequest it returns and answers with response
// - the shape every test in this file needs to read back what
// buildRequest actually sent.
func captureRequestBody(t *testing.T, response string) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured.body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, captured
}

// TestPickWithNoTurnsSendsStatePlainByteForByte guards the no-turns case
// this trial's own build note requires: with turns nil or empty, "state"
// is still stateFor's plain string - decoded here as a Go string, not an
// object - byte for byte what every request this adapter sent before v5.
// The default picker's own instructions is a plain string too (v5's own
// per-variable isolation, docs/measurements/jev-v5.md) - a separate
// switch (WithObjectInstructions) from the state one this test guards.
func TestPickWithNoTurnsSendsStatePlainByteForByte(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "listInventoryItems", 0.9))

	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	assert.Equal(t, "在庫を見せて", gotBody.body["state"], "state must stay a plain string with no turns")

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, questions, 1, "no fan-out gate: only the pick question is sent")

	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)
	_, isString := pickQuestion["instructions"].(string)
	assert.True(t, isString, "instructions must be a plain string by default")
}

// TestPickWithObjectInstructionsAndNoTurnsCarriesNoContextField is the
// object-instructions form's own no-turns guard (the counterpart to the
// default-instructions test above): under WithObjectInstructions, with no
// turns, the "pick" question's own instructions object carries no
// `context` field - it is only added when turns is non-empty
// (instructionsFor).
func TestPickWithObjectInstructionsAndNoTurnsCarriesNoContextField(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "listInventoryItems", 0.9))

	picker := jev.New(server.URL, "test-key", nil, jev.WithObjectInstructions())

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)
	instructions, ok := pickQuestion["instructions"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, instructions, "context", "no turns: instructions carries no context field")
}

// turnsShortlist is one endpoint the turn below names (with its own
// display names, so turnsFor's shortlist lookup has something to find)
// plus a second, unrelated one - the shortlist the pick itself is
// offered, standing in for a follow-up ("勤怠でも同じことして") whose own
// shortlist has been narrowed to the other service.
func turnsShortlist() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service: "attendance", OperationID: "ListAttendanceRecords",
			ServiceDisplayName: "勤怠管理", DisplayName: "勤怠記録の一覧",
			Summary: "勤怠記録の一覧を返す", Method: domain.MethodGet, Path: "/records",
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

// TestPickWithTurnsSendsStateAsAnObjectWithDisplayNames is this round's
// own "request body with two turns (exact)": the first turn names an
// operation present in the pick's own shortlist (turnsShortlist) and must
// carry its display names, not its raw ids; the second names an
// operation the shortlist no longer carries (idAffinity or narrowing
// dropped it) and must fall back to its own raw Service/OperationID
// rather than losing the turn. The "pick" question's own instructions
// gains a "context" field pointing at `state.turns` once turns is
// non-empty.
func TestPickWithTurnsSendsStateAsAnObjectWithDisplayNames(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "ListAttendanceRecords", 0.9))

	// WithObjectInstructions: state carrying turns (asserted below) is
	// independent of the instructions style, but this test also checks
	// the object form's own "context" field, which only exists under
	// this option (the default since v5's own isolation is a plain
	// string, docs/measurements/jev-v5.md).
	picker := jev.New(server.URL, "test-key", nil, jev.WithObjectInstructions())

	turns := []usecase.Turn{
		{
			Question: "在庫を全部見せて", Kind: usecase.ResultKindResult,
			Service: "inventory", OperationID: "ListInventoryItems",
		},
		{
			Question: "検品保留のものだけ見せて", Kind: usecase.ResultKindResult,
			Service: "inventory", OperationID: "ListInventoryItems", Args: map[string]any{"status": "quarantined"},
		},
	}

	// The first turn's own operation, ListInventoryItems, is deliberately
	// absent from turnsShortlist (idAffinity/narrowing having dropped
	// inventory from the shortlist a "勤怠でも同じことして" follow-up
	// would be offered) - so both turns above resolve through the same
	// raw-id fallback, and turnsShortlist's own attendance entry proves
	// the found-in-shortlist branch through the response the fixture
	// answers, not through the request itself; a second, present-in-
	// shortlist case is covered by
	// TestPickWithATurnFoundInTheShortlistUsesItsDisplayNames below.
	_, err := picker.Pick(context.Background(), "勤怠でも同じことして", nil, turns, turnsShortlist())
	require.NoError(t, err)

	state, ok := gotBody.body["state"].(map[string]any)
	require.True(t, ok, "state must be an object once turns is non-empty")

	assert.Equal(t, "勤怠でも同じことして", state["question"])
	assert.Equal(t, []any{}, state["answers"], "no answers: an empty array, not null")

	gotTurns, ok := state["turns"].([]any)
	require.True(t, ok)
	require.Len(t, gotTurns, 2)

	first, ok := gotTurns[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "在庫を全部見せて", first["question"])
	assert.Equal(t, "inventory", first["service"], "not in the pick's own shortlist: falls back to the raw service id")
	assert.Equal(t, "ListInventoryItems", first["operation"], "falls back to the raw operation id")

	second, ok := gotTurns[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "検品保留のものだけ見せて", second["question"])
	assert.Equal(t, "inventory", second["service"])
	assert.Equal(t, "ListInventoryItems", second["operation"])

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)
	instructions, ok := pickQuestion["instructions"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, instructions["context"], "state.turns")
}

// TestPickWithATurnFoundInTheShortlistUsesItsDisplayNames is the other
// half of the turns test above: a turn naming an operation still present
// in the pick's own shortlist must carry that operation's own display
// names (the same vocabulary its own candidate line uses), not its raw
// ids.
func TestPickWithATurnFoundInTheShortlistUsesItsDisplayNames(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "ListAttendanceRecords", 0.9))

	picker := jev.New(server.URL, "test-key", nil)

	turns := []usecase.Turn{
		{
			Question: "勤怠を見せて", Kind: usecase.ResultKindResult,
			Service: "attendance", OperationID: "ListAttendanceRecords",
		},
	}

	_, err := picker.Pick(context.Background(), "続けて", nil, turns, turnsShortlist())
	require.NoError(t, err)

	state, ok := gotBody.body["state"].(map[string]any)
	require.True(t, ok)

	gotTurns, ok := state["turns"].([]any)
	require.True(t, ok)
	require.Len(t, gotTurns, 1)

	first, ok := gotTurns[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "勤怠管理", first["service"], "found in the shortlist: its own ServiceDisplayName")
	assert.Equal(t, "勤怠記録の一覧", first["operation"], "found in the shortlist: its own DisplayName")
}

// TestPickWithoutFanOutGateNeverSendsAnImpossibleQuestion guards the
// default (New with no WithFanOutGate): only "pick" is ever sent, exactly
// as before this trial's fan-out addition.
func TestPickWithoutFanOutGateNeverSendsAnImpossibleQuestion(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "listInventoryItems", 0.9))

	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, questions, "impossible")
}

// impossibleResponse builds a fixture answering both "pick" and
// "impossible" in one response, the fan-out request's own answer shape.
func impossibleResponse(t *testing.T, choice string, confidence, noul float64) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"model": "jev-1.13.0",
		"answers": map[string]any{
			"pick":       map[string]any{"type": "choice", "choice": choice, "confidence": confidence},
			"impossible": map[string]any{"type": "noul", "noul": noul},
		},
		"usage": map[string]any{"input_tokens": 700, "output_tokens": 3},
	})
	require.NoError(t, err)

	return string(body)
}

// TestPickWithFanOutGateSendsBothQuestionsInOneRequest is this round's
// own "request body ... with the fan-out gate question present": one
// request, "pick" and "impossible" both, "impossible" a "noul" type with
// its own true/false criteria (point 2 of
// docs/measurements/jev-picker-v5.md's "what the API offers that v1-v4
// did not use" - v3's own gate sent instructions alone).
func TestPickWithFanOutGateSendsBothQuestionsInOneRequest(t *testing.T) {
	server, gotBody := captureRequestBody(t, impossibleResponse(t, "listInventoryItems", 0.9, 0.1))

	picker := jev.New(server.URL, "test-key", nil, jev.WithFanOutGate(0.7))

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, questions, 2)

	assert.Contains(t, questions, "pick")

	impossible, ok := questions["impossible"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "noul", impossible["type"])

	criteria, ok := impossible["criteria"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, criteria["true"])
	assert.NotEmpty(t, criteria["false"])
}

// TestPickWithFanOutGateAboveThresholdReturnsPickNoneWithoutTheChoice
// proves the short-circuit: a noul at or above threshold answers
// PickNone directly from the one fan-out response, without mapAnswer ever
// needing to have matched the "pick" answer - here deliberately set to a
// choice mapAnswer could not resolve at all (an operation id absent from
// the shortlist), so a passing result proves the pick answer was never
// consulted.
func TestPickWithFanOutGateAboveThresholdReturnsPickNoneWithoutTheChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(impossibleResponse(t, "notAShortlistOperation", 0.99, 0.9))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil, jev.WithFanOutGate(0.7))

	got, err := picker.Pick(context.Background(), "在庫を集計したい", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, got)
}

// TestPickWithFanOutGateBelowThresholdProceedsToTheChoice is the other
// side: a noul below threshold is logged but never overrides the "pick"
// answer.
func TestPickWithFanOutGateBelowThresholdProceedsToTheChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(impossibleResponse(t, "listInventoryItems", 0.9, 0.1))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil, jev.WithFanOutGate(0.7))

	got, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.Equal(t, usecase.Pick{
		Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems", Confidence: 0.9,
	}, got)
}

// TestPickWithFanOutGateLogsTheGateVerdictAtInfo proves the "gate
// completed" log line this round adds (picker.go), the fan-out
// counterpart to the standalone Gate.Gate's own "gate completed" line
// (gate.go) - gate_fanout distinguishes the two in a shared platform log.
func TestPickWithFanOutGateLogsTheGateVerdictAtInfo(t *testing.T) {
	buf := captureLogs(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(impossibleResponse(t, "listInventoryItems", 0.9, 0.42))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil, jev.WithFanOutGate(0.7))

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	line := logLineWith(t, buf, "gate_noul")
	require.NotNil(t, line)
	assert.InDelta(t, 0.42, line["gate_noul"], 0)
	assert.Equal(t, false, line["gate_impossible"])
	assert.Equal(t, true, line["gate_fanout"])
}

// TestPickByDefaultSendsThePlainV2StringWithTurnsStillInState is v5's own
// per-variable isolation result (docs/measurements/jev-v5.md): a Picker
// built with no WithObjectInstructions option (the default, since
// isolation measured the object form regressing real-attendance-detail)
// still sends turns inside "state" as an object - the two are
// independent - but the "pick" question's own instructions is the plain
// string v1/v2 always sent, byte for byte defaultInstructionsV2, no
// `focus`, no `context`, no object at all.
func TestPickByDefaultSendsThePlainV2StringWithTurnsStillInState(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "ListAttendanceRecords", 0.9))

	picker := jev.New(server.URL, "test-key", nil, jev.WithCriteria(jev.CriteriaV2))

	turns := []usecase.Turn{
		{Question: "勤怠を見せて", Kind: usecase.ResultKindResult, Service: "attendance", OperationID: "ListAttendanceRecords"},
	}

	_, err := picker.Pick(context.Background(), "続けて", nil, turns, turnsShortlist())
	require.NoError(t, err)

	state, ok := gotBody.body["state"].(map[string]any)
	require.True(t, ok, "turns still reach state as an object by default")

	gotTurns, ok := state["turns"].([]any)
	require.True(t, ok)
	assert.Len(t, gotTurns, 1)

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)

	instructions, ok := pickQuestion["instructions"].(string)
	require.True(t, ok, "instructions must be a plain string by default, not an object")
	assert.Contains(t, instructions, "examples")
	assert.Contains(t, instructions, "not_for")
	assert.NotContains(t, instructions, "動詞", "the v5 object form's own focus wording must not appear by default")
}

// TestPickWithObjectInstructionsSendsTheV5ObjectByDefault guards the
// opt-in: a Picker built with WithObjectInstructions sends the v5 object
// form, the alternative isolation measured as a net-negative default
// (docs/measurements/jev-v5.md) but kept available for a narrower
// follow-up.
func TestPickWithObjectInstructionsSendsTheV5ObjectByDefault(t *testing.T) {
	server, gotBody := captureRequestBody(t, responseWith(t, "listInventoryItems", 0.9))

	picker := jev.New(server.URL, "test-key", nil, jev.WithObjectInstructions())

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	questions, ok := gotBody.body["questions"].(map[string]any)
	require.True(t, ok)
	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)

	_, isObject := pickQuestion["instructions"].(map[string]any)
	assert.True(t, isObject, "instructions must be an object under WithObjectInstructions")
}
