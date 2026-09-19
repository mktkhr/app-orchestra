package toolcall_test

// Split from planner_test.go (harness/guard/filelen.sh - 1000-line file
// cap): the toolcall.WithThinking / toolcall.WithRepeatPenalty tests and
// the truncation warn log's field tests (platform knobs subproject,
// 2026-09-16). fixtureCatalog, newPlanner, stubServer, callResponse and
// lengthResponse are all defined in planner_test.go, same package.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// TestPlanWithThinkingLeftOnSendsNoChatTemplateKwargs is the other half of
// WithThinking(false)'s contract: an option-less Planner (today's
// behaviour) sends no "chat_template_kwargs" at all, so Qwen3.5's
// thinking stays on by default (DECISIONS.md; probed 2026-09-16).
func TestPlanWithThinkingLeftOnSendsNoChatTemplateKwargs(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	_, ok := gotBody["chat_template_kwargs"]
	assert.False(t, ok, "chat_template_kwargs must be absent when thinking is left on")
}

// TestPlanWithThinkingDisabledSendsEnableThinkingFalse documents
// WithThinking(false): every planning request carries
// chat_template_kwargs: {"enable_thinking": false}.
func TestPlanWithThinkingDisabledSendsEnableThinkingFalse(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog(), toolcall.WithThinking(false))

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	kwargs, ok := gotBody["chat_template_kwargs"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, kwargs["enable_thinking"])
}

// thinkingTrue and thinkingFalse are *bool literals a test can pass as
// Plan's own thinking parameter - a per-request override, distinct from
// the Planner's own configured default set by WithThinking (platform
// knobs subproject, decided 2026-09-16).
func thinkingTrue() *bool {
	v := true

	return &v
}

func thinkingFalse() *bool {
	v := false

	return &v
}

// TestPlanPerRequestThinkingTrueOverridesConfiguredFalse is one direction
// of the effective-value rule Plan's own doc comment states: a Planner
// configured with WithThinking(false) still sends no
// "chat_template_kwargs" when the request itself asks for thinking: true.
func TestPlanPerRequestThinkingTrueOverridesConfiguredFalse(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog(), toolcall.WithThinking(false))

	_, err := planner.Plan(
		context.Background(), "何か", nil, nil,
		usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), thinkingTrue(),
	)
	require.NoError(t, err)

	_, ok := gotBody["chat_template_kwargs"]
	assert.False(t, ok, "chat_template_kwargs must be absent when the request overrides thinking to true")
}

// TestPlanPerRequestThinkingFalseOverridesConfiguredTrue is the other
// direction: a Planner with no WithThinking option at all (thinking on by
// default) still sends chat_template_kwargs: {"enable_thinking": false}
// when the request itself asks for thinking: false.
func TestPlanPerRequestThinkingFalseOverridesConfiguredTrue(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(
		context.Background(), "何か", nil, nil,
		usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), thinkingFalse(),
	)
	require.NoError(t, err)

	kwargs, ok := gotBody["chat_template_kwargs"].(map[string]any)
	require.True(t, ok, "chat_template_kwargs must be present when the request overrides thinking to false")
	assert.Equal(t, false, kwargs["enable_thinking"])
}

// TestPlanDebugLogReportsTheEffectiveThinkingValueNotTheConfiguredDefault
// is the logging half of the effective-value rule: the "thinking" field on
// the per-call debug log is the request's own override, not the Planner's
// configured default, whenever the two disagree.
func TestPlanDebugLogReportsTheEffectiveThinkingValueNotTheConfiguredDefault(t *testing.T) {
	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	planner := newPlanner(t, callResponse, fixtureCatalog())

	_, err := planner.Plan(
		context.Background(), "何か", nil, nil,
		usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), thinkingFalse(),
	)
	require.NoError(t, err)

	completed := logLineWith(t, buf, "finish_reason")
	require.NotNil(t, completed, "expected a debug log carrying \"finish_reason\"")
	assert.Equal(t, false, completed["thinking"],
		"the logged thinking value must be the request's override, not the configured default (true)")
}

// TestPlanWithNoRepeatPenaltySendsNeitherField documents that an
// option-less Planner (today's behaviour) sends neither "repeat_penalty"
// nor "repeat_last_n".
func TestPlanWithNoRepeatPenaltySendsNeitherField(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	_, hasPenalty := gotBody["repeat_penalty"]
	_, hasLastN := gotBody["repeat_last_n"]
	assert.False(t, hasPenalty, "repeat_penalty must be absent when WithRepeatPenalty was not used")
	assert.False(t, hasLastN, "repeat_last_n must be absent when WithRepeatPenalty was not used")
}

// TestPlanWithRepeatPenaltySendsBothFields documents
// WithRepeatPenalty(1.1, 64): every planning request carries both fields.
func TestPlanWithRepeatPenaltySendsBothFields(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog(), toolcall.WithRepeatPenalty(1.1, 64))

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.InDelta(t, 1.1, gotBody["repeat_penalty"], 0)
	assert.InDelta(t, 64.0, gotBody["repeat_last_n"], 0)
}

// lengthResponseWithReasoning mirrors lengthResponse but with a
// "reasoning_content" and a "usage.completion_tokens" on it, the shape
// thinking-that-never-finished actually takes (facts, 2026-09-16) -
// exercising the truncation warn log's new fields.
const lengthResponseWithReasoning = `{
  "choices": [{
    "finish_reason": "length",
    "message": {
      "role": "assistant",
      "reasoning_content": "Thinking Process: let me consider the tools available..."
    }
  }],
  "usage": {"completion_tokens": 1024}
}`

// captureLogs installs a slog.NewJSONHandler over buf as slog's
// process-wide default for the duration of t, restoring the previous one
// on cleanup - toolcall.Planner logs through slog.Default() (see Plan),
// not a logger threaded through it, so this is the only seam a test has.
// A real handler (rather than a hand-rolled slog.Handler, whose Handle
// method would have to accept slog.Record by value - gocritic's
// hugeParam, harness/quality/go/golangci.yml) means each logged line is
// read back the same way a person reading the platform's own JSON logs
// would.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return buf
}

// logLineWith returns the first JSON log line in buf carrying key, or nil
// if none does.
func logLineWith(t *testing.T, buf *bytes.Buffer, key string) map[string]any {
	t.Helper()

	for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
		if line == "" {
			continue
		}

		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))

		if _, ok := record[key]; ok {
			return record
		}
	}

	return nil
}

// TestPlanWithNoMaxTokensSends1024 documents that an option-less Planner
// (today's behaviour) sends max_tokens: 1024, chat.MaxTokens's own fixed
// value - unchanged since ORCHESTRA_PLANNER_MAX_TOKENS started overriding
// it.
func TestPlanWithNoMaxTokensSends1024(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.InDelta(t, 1024.0, gotBody["max_tokens"], 0)
}

// TestPlanWithMaxTokensOverridesTheDefault documents WithMaxTokens: a
// thinking model needs a bigger budget than chat.MaxTokens's fixed 1024
// (measured 2026-09-19, docs/specs/shortlisting.md).
func TestPlanWithMaxTokensOverridesTheDefault(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(callResponse)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog(), toolcall.WithMaxTokens(4000))

	_, err := planner.Plan(context.Background(), "何か", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	assert.InDelta(t, 4000.0, gotBody["max_tokens"], 0)
}

// TestPlanTruncationLogCarriesReasoningCompletionTokensAndThinking is the
// evidence half of the facts recorded 2026-09-16: today, a truncated
// answer's warn log carries only "content", empty whenever thinking
// consumed the whole budget - reasoning, completion_tokens and thinking
// are what let the next person tell that case apart from a plain
// repetition loop without reproducing the run.
func TestPlanTruncationLogCarriesReasoningCompletionTokensAndThinking(t *testing.T) {
	buf := captureLogs(t)
	planner := newPlanner(t, lengthResponseWithReasoning, fixtureCatalog())

	_, err := planner.Plan(context.Background(), "明細を1件確認したい", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil)
	require.NoError(t, err)

	warn := logLineWith(t, buf, "reasoning")

	require.NotNil(t, warn, "expected a warn log carrying \"reasoning\"")
	assert.Equal(t, "Thinking Process: let me consider the tools available...", warn["reasoning"])
	assert.InDelta(t, 1024.0, warn["completion_tokens"], 0)
	assert.Equal(t, true, warn["thinking"])
}
