package pick_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// shortlistCatalog is two endpoints on two services, in a deliberately
// non-alphabetical order, standing in for a reranker's shortlist.
func shortlistCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:            "inventory",
			OperationID:        "listInventoryItems",
			ServiceDisplayName: "在庫管理",
			Summary:            "在庫の一覧を返す",
			Method:             domain.MethodGet,
			Path:               "/items",
			Response:           &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service:     "attendance",
			OperationID: "listAttendanceRecords",
			Summary:     "勤怠記録の一覧を返す",
			Method:      domain.MethodGet,
			Path:        "/records",
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

func stubServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func newPicker(t *testing.T, body string) *pick.Picker {
	t.Helper()

	server := stubServer(t, body)
	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})

	return pick.New(client, "qwen3.5-9b-q8")
}

func responseWith(content string) string {
	return `{"choices": [{"finish_reason": "stop", "message": {"role": "assistant", "content": "` + content + `"}}]}`
}

func TestPickSendsTheExactRequestTheStandInPickerSent(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith("listInventoryItems certain"))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	picker := pick.New(client, "qwen3.5-9b-q8")

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	temperature, ok := gotBody["temperature"].(float64)
	require.True(t, ok)
	maxTokens, ok := gotBody["max_tokens"].(float64)
	require.True(t, ok)

	assert.Equal(t, "qwen3.5-9b-q8", gotBody["model"])
	assert.InDelta(t, 0.0, temperature, 0)
	assert.InDelta(t, 200.0, maxTokens, 0)
	assert.Equal(t, map[string]any{"enable_thinking": false}, gotBody["chat_template_kwargs"])
	assert.Nil(t, gotBody["tools"], "the pick never offers tools - it is not tool calling")

	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	system, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "system", system["role"])
	assert.Equal(t, pick.SystemPrompt, system["content"])

	user, ok := messages[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", user["role"])
	assert.Equal(t,
		"質問: 在庫を見せて\n\n候補:\n"+
			"listInventoryItems\t在庫管理\t在庫の一覧を返す\n"+
			"listAttendanceRecords\tattendance\t勤怠記録の一覧を返す\n"+
			"list_capabilities\tplatform\t使える操作の一覧そのものを求めている\n"+
			"propose_panel\tplatform\t画面に出したい\n"+
			"none\tplatform\tどの候補も質問に合わない（業務と無関係な質問、候補の操作では答えられない質問）",
		user["content"],
	)
}

// TestPickWithNoWorkspaceOmitsProposePanelFromTheCandidateList is O3
// (docs/specs/offering.md): a request carrying no workspace
// (usecase.PlanContext{} - the zero value, WorkspaceID "") must never see
// propose_panel among its candidates, in the user message or anywhere else
// a response could match it against - list_capabilities and none stay,
// unconditionally, exactly as usecase.ToolsFor already excludes the
// built-in tool of the same name for planOrdinary/planPreferred rather than
// merely describing it as unavailable.
func TestPickWithNoWorkspaceOmitsProposePanelFromTheCandidateList(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith("listInventoryItems certain"))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	picker := pick.New(client, "qwen3.5-9b-q8")

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog(), usecase.PlanContext{})
	require.NoError(t, err)

	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	user, ok := messages[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t,
		"質問: 在庫を見せて\n\n候補:\n"+
			"listInventoryItems\t在庫管理\t在庫の一覧を返す\n"+
			"listAttendanceRecords\tattendance\t勤怠記録の一覧を返す\n"+
			"list_capabilities\tplatform\t使える操作の一覧そのものを求めている\n"+
			"none\tplatform\tどの候補も質問に合わない（業務と無関係な質問、候補の操作では答えられない質問）",
		user["content"],
	)
	assert.NotContains(t, user["content"], "propose_panel")
}

// TestPickWithNoWorkspaceNeverParsesProposePanel is
// TestPickWithNoWorkspaceOmitsProposePanelFromTheCandidateList's other
// half: even if the model answers "propose_panel" anyway (it was never
// offered it, but nothing stops a model from naming an id it has seen
// before in training or a prior turn's own system prompt), the pick must
// not match it - candidatesFor never lists it as a candidate, so parse
// falls through to PickNone exactly as it would for any other unrecognised
// id.
func TestPickWithNoWorkspaceNeverParsesProposePanel(t *testing.T) {
	picker := newPicker(t, responseWith("propose_panel certain"))

	got, err := picker.Pick(context.Background(), "画面に置いて", nil, nil, shortlistCatalog(), usecase.PlanContext{})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone, Ambiguous: false}, got)
}

// captureRequestBody drives one Pick call with the given turns against a
// fixture server and returns the raw request body it sent - the shared
// setup TestPickByteIdenticalWithNoTurns and
// TestPickRendersTurnsIntoTheUserMessage both need, factored out so golangci
// (dupl) sees one copy, not two.
func captureRequestBody(t *testing.T, turns []usecase.Turn) string {
	t.Helper()

	var got string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		got = string(raw)

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith("listInventoryItems certain"))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	picker := pick.New(client, "qwen3.5-9b-q8")

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, turns, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	return got
}

// TestPickByteIdenticalWithNoTurns is TestPickIgnoresTurns narrowed to what
// is still true since docs/measurements/jev-v5.md's isolation result:
// userMessage now renders turns (turnLines, prompt.go), so Pick's request
// body is no longer byte-identical whenever turns carries entries - but a
// request built with an explicitly empty, non-nil turns slice must still be
// byte-identical to one built with nil, the same guarantee answers already
// has (TestUserMessageWithNoAnswersOrTurnsIsByteIdenticalToBeforeTheyExisted,
// prompt_internal_test.go). SystemPrompt is unaffected either way - it must
// stay byte-identical to e2e/narrowing/pick/client.ts's own
// PICK_SYSTEM_PROMPT (S2's cross-language comparison, prompt_test.go).
func TestPickByteIdenticalWithNoTurns(t *testing.T) {
	assert.Equal(t, captureRequestBody(t, nil), captureRequestBody(t, []usecase.Turn{}),
		"an empty, non-nil turns slice must build the same request as nil")
}

// TestPickRendersTurnsIntoTheUserMessage is the with-turns counterpart:
// a request built with turns carries turnLines' "直前: ..." line and so
// differs from one built without, proving Pick no longer discards turns the
// way it did before docs/measurements/jev-v5.md's isolation result (the
// Jev picker recovered follow-up-other-service from 0/10 to 10/10 by
// seeing them; the local pick had the same blindness).
func TestPickRendersTurnsIntoTheUserMessage(t *testing.T) {
	withoutTurns := captureRequestBody(t, nil)
	withTurns := captureRequestBody(t, []usecase.Turn{
		{Question: "前の質問", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "listInventoryItems"},
	})

	assert.NotEqual(t, withoutTurns, withTurns)
	assert.Contains(t, withTurns, "直前:")
	assert.Contains(t, withTurns, "前の質問")
}

func TestPickParsesACertainOperation(t *testing.T) {
	picker := newPicker(t, responseWith("listInventoryItems certain"))

	got, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems"}, got)
}

func TestPickParsesAnAmbiguousOperation(t *testing.T) {
	picker := newPicker(t, responseWith("listInventoryItems ambiguous"))

	got, err := picker.Pick(context.Background(), "見せて", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{
		Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems", Ambiguous: true,
	}, got)
}

// TestPickPicksTheLongestIdWhenOneIsASubstringOfAnother is the substring
// guard: listInventoryItem must not steal listInventoryItems's match.
func TestPickPicksTheLongestIdWhenOneIsASubstringOfAnother(t *testing.T) {
	shortlist := domain.Catalog{Endpoints: []domain.Endpoint{
		{Service: "inventory", OperationID: "listInventoryItem", Summary: "一件を返す", Method: domain.MethodGet, Path: "/i", Response: &domain.Schema{Type: domain.SchemaTypeObject}},
		{Service: "inventory", OperationID: "listInventoryItems", Summary: "一覧を返す", Method: domain.MethodGet, Path: "/is", Response: &domain.Schema{Type: domain.SchemaTypeObject}},
	}}

	picker := newPicker(t, responseWith("listInventoryItems certain"))

	got, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlist, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, "listInventoryItems", got.OperationID)
}

func TestPickParsesListCapabilities(t *testing.T) {
	picker := newPicker(t, responseWith("list_capabilities certain"))

	got, err := picker.Pick(context.Background(), "何ができるの？", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickListCapabilities}, got)
}

func TestPickParsesProposePanel(t *testing.T) {
	picker := newPicker(t, responseWith("propose_panel certain"))

	got, err := picker.Pick(context.Background(), "画面に置いて", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickProposePanel}, got)
}

func TestPickParsesNoneExplicitly(t *testing.T) {
	picker := newPicker(t, responseWith("none certain"))

	got, err := picker.Pick(context.Background(), "今日の天気は？", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, got)
}

func TestPickWithNoRecognisableIdAtAllIsNone(t *testing.T) {
	picker := newPicker(t, responseWith("わかりません"))

	got, err := picker.Pick(context.Background(), "今日の天気は？", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, got)
}

// TestPickTruncatedByMaxTokensIsNone mirrors
// toolcall.Planner's own TestPlanMapsATruncatedAnswerOntoDecisionNone: a
// response cut off by max_tokens is never decoded as if it were real.
func TestPickTruncatedByMaxTokensIsNone(t *testing.T) {
	body := `{"choices": [{"finish_reason": "length", "message": {"role": "assistant", "content": "listInventoryIt"}}]}`
	picker := newPicker(t, body)

	got, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, got)
}

// TestPickWrapsATransportError proves a chat.Client error reaches the
// caller, wrapped, rather than being swallowed.
func TestPickWrapsATransportError(t *testing.T) {
	client := chat.New(chat.Config{BaseURL: "http://127.0.0.1:0", Model: "test-model"})
	picker := pick.New(client, "qwen3.5-9b-q8")

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"})
	require.Error(t, err)
}

// TestPickCandidateLineFallsBackToTheDescriptionsFirstLineWhenSummaryIsEmpty
// is summaryFor's own fallback (prompt.go): an endpoint with no Summary at
// all still gets a usable candidate line, from its Description's first
// line.
func TestPickCandidateLineFallsBackToTheDescriptionsFirstLineWhenSummaryIsEmpty(t *testing.T) {
	shortlist := domain.Catalog{Endpoints: []domain.Endpoint{{
		Service:     "inventory",
		OperationID: "listInventoryItems",
		Description: "在庫の一覧を返す\nさらに詳しい説明が続く",
		Method:      domain.MethodGet,
		Path:        "/items",
		Response:    &domain.Schema{Type: domain.SchemaTypeObject},
	}}}

	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith("listInventoryItems certain"))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	picker := pick.New(client, "qwen3.5-9b-q8")

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlist, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	user, ok := messages[1].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, user["content"], "listInventoryItems\tinventory\t在庫の一覧を返す\n")
}

// TestPickOnEmptyShortlistNeverCallsTheModel proves the fast path: nothing
// to pick from is PickNone without a request at all.
func TestPickOnEmptyShortlistNeverCallsTheModel(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith("none certain"))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	picker := pick.New(client, "qwen3.5-9b-q8")

	got, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, domain.Catalog{}, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, got)
	assert.Zero(t, calls, "an empty shortlist must never reach the model")
}
