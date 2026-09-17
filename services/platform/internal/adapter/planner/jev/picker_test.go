package jev_test

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

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// shortlistCatalog is two endpoints on two services, the same fixture
// internal/adapter/planner/pick/picker_test.go uses, so both pickers are
// measured against the same shortlist shape.
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

func stubServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

func responseWith(t *testing.T, choice string, confidence float64) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"model": "jev-1.13.0",
		"answers": map[string]any{
			"pick": map[string]any{
				"type":       "choice",
				"choice":     choice,
				"confidence": confidence,
			},
		},
		"usage": map[string]any{"input_tokens": 700, "output_tokens": 3},
	})
	require.NoError(t, err)

	return string(body)
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return buf
}

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

func TestPickSendsStateInstructionsAndCriteria(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith(t, "listInventoryItems", 0.9))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	assert.Equal(t, "在庫を見せて", gotBody["state"])
	assert.Equal(t, "jev-latest", gotBody["model"])

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "choice", pickQuestion["type"])

	// instructions is the plain-string form by default (v5's own
	// per-variable isolation, docs/measurements/jev-v5.md).
	instructions, ok := pickQuestion["instructions"].(string)
	require.True(t, ok)
	assert.Contains(t, instructions, "list_capabilities")
	assert.Contains(t, instructions, "propose_panel")
	assert.Contains(t, instructions, "none")

	criteria, ok := pickQuestion["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "在庫管理 / 在庫の一覧を返す", criteria["listInventoryItems"])
	assert.Equal(t, "attendance / 勤怠記録の一覧を返す", criteria["listAttendanceRecords"])
	assert.Equal(t, pick.PhraseListCapabilities, criteria[pick.IDListCapabilities])
	assert.Equal(t, pick.PhraseProposePanel, criteria[pick.IDProposePanel])
	assert.Equal(t, pick.PhraseNone, criteria[pick.IDNone])
}

func TestPickAppendsOneAnswerLinePerAnswer(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseWith(t, "listInventoryItems", 0.9))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil)

	answers := []usecase.Answer{{Param: "status", Value: "active"}, {Param: "type", Value: "A"}}

	_, err := picker.Pick(context.Background(), "在庫を見せて", answers, nil, shortlistCatalog())
	require.NoError(t, err)

	assert.Equal(t, "在庫を見せて\n回答: status=active\n回答: type=A", gotBody["state"])
}

func TestPickMapsChoiceToShortlistEndpoint(t *testing.T) {
	server := stubServer(t, http.StatusOK, responseWith(t, "listAttendanceRecords", 0.9))
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "勤怠を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.Equal(t, usecase.Pick{
		Kind: usecase.PickOperation, Service: "attendance", OperationID: "listAttendanceRecords", Ambiguous: false,
	}, result)
}

func TestPickMapsEachBuiltinChoice(t *testing.T) {
	tests := map[string]usecase.PickKind{
		pick.IDListCapabilities: usecase.PickListCapabilities,
		pick.IDProposePanel:     usecase.PickProposePanel,
		pick.IDNone:             usecase.PickNone,
	}

	for choice, want := range tests {
		t.Run(choice, func(t *testing.T) {
			server := stubServer(t, http.StatusOK, responseWith(t, choice, 0.9))
			picker := jev.New(server.URL, "test-key", nil)

			result, err := picker.Pick(context.Background(), "何ができる？", nil, nil, shortlistCatalog())
			require.NoError(t, err)
			assert.Equal(t, want, result.Kind)
			assert.False(t, result.Ambiguous)
		})
	}
}

func TestPickConfidenceBelowThresholdIsAmbiguous(t *testing.T) {
	server := stubServer(t, http.StatusOK, responseWith(t, "listInventoryItems", 0.4))
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.True(t, result.Ambiguous)
}

func TestPickConfidenceAtOrAboveThresholdIsNotAmbiguous(t *testing.T) {
	server := stubServer(t, http.StatusOK, responseWith(t, "listInventoryItems", 0.5))
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.False(t, result.Ambiguous)
}

func TestPickWithAmbiguityThresholdOverridesTheDefault(t *testing.T) {
	server := stubServer(t, http.StatusOK, responseWith(t, "listInventoryItems", 0.8))
	picker := jev.New(server.URL, "test-key", nil, jev.WithAmbiguityThreshold(0.9))

	result, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.True(t, result.Ambiguous)
}

func TestPickUnauthorizedNamesTheStatusWithoutTheKey(t *testing.T) {
	server := stubServer(t, http.StatusUnauthorized, `{"error":"invalid api key"}`)
	picker := jev.New(server.URL, "super-secret-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.NotContains(t, err.Error(), "super-secret-key")
}

func TestPickUnknownChoiceIsPickNoneWithAWarnLog(t *testing.T) {
	buf := captureLogs(t)
	server := stubServer(t, http.StatusOK, responseWith(t, "noSuchOperation", 0.9))
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.Equal(t, usecase.PickNone, result.Kind)

	warn := logLineWith(t, buf, "choice")
	require.NotNil(t, warn, "expected a warn log naming the unknown choice")
	assert.Equal(t, "noSuchOperation", warn["choice"])
}

func TestPickEmptyShortlistIsPickNoneWithoutACall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, result)
	assert.False(t, called, "expected no call to the API for an empty shortlist")
}

func TestPickCompletedLogCarriesConfidenceTokensAndProvider(t *testing.T) {
	buf := captureLogs(t)
	server := stubServer(t, http.StatusOK, responseWith(t, "listInventoryItems", 0.87))
	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	completed := logLineWith(t, buf, "pick_confidence")
	require.NotNil(t, completed, "expected a \"pick completed\" log line")
	assert.InDelta(t, 0.87, completed["pick_confidence"], 0.0001)
	assert.InDelta(t, float64(700), completed["pick_input_tokens"], 0)
	assert.InDelta(t, float64(3), completed["pick_output_tokens"], 0)
	assert.Equal(t, "jev", completed["pick_provider"])
	assert.Equal(t, "listInventoryItems", completed["pick_operation_id"])
	assert.Equal(t, "listInventoryItems", completed["pick_choice"])
}

// TestPickCompletedLogCarriesTheRawChoiceForABuiltin documents why
// pick_choice exists alongside pick_operation_id: a built-in kind leaves
// pick_operation_id empty (usecase.Pick's own shape - see mapAnswer), so
// pick_choice is the only field in the log line that still names what
// Jev actually answered.
func TestPickCompletedLogCarriesTheRawChoiceForABuiltin(t *testing.T) {
	buf := captureLogs(t)
	server := stubServer(t, http.StatusOK, responseWith(t, pick.IDNone, 0.9))
	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "今日の天気は？", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	completed := logLineWith(t, buf, "pick_choice")
	require.NotNil(t, completed, "expected a \"pick completed\" log line")
	assert.Equal(t, pick.IDNone, completed["pick_choice"])
	assert.Empty(t, completed["pick_operation_id"])
}

// TestPickCompletedLogCarriesTheFullProbabilityDistribution is the Jev
// trial measurement's own need (2026-09-17): a run's platform log alone
// must be enough to reconstruct each pick's full per-candidate
// distribution, not just the winning choice's own confidence.
func TestPickCompletedLogCarriesTheFullProbabilityDistribution(t *testing.T) {
	buf := captureLogs(t)
	body := `{"model":"jev-1.13.0","answers":{"pick":{"type":"choice","choice":"listInventoryItems",` +
		`"confidence":0.87,"probabilities":{"listInventoryItems":0.87,"listAttendanceRecords":0.1,"none":0.03}}},` +
		`"usage":{"input_tokens":700,"output_tokens":3}}`
	server := stubServer(t, http.StatusOK, body)
	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	completed := logLineWith(t, buf, "pick_probabilities")
	require.NotNil(t, completed, "expected a \"pick completed\" log line carrying pick_probabilities")

	probabilities, ok := completed["pick_probabilities"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.87, probabilities["listInventoryItems"], 0.0001)
	assert.InDelta(t, 0.1, probabilities["listAttendanceRecords"], 0.0001)
	assert.InDelta(t, 0.03, probabilities["none"], 0.0001)
}
