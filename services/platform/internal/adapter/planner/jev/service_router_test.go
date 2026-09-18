package jev_test

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
)

// routeResponseWith builds a fixed "route" answer body - this file's own
// counterpart to picker_test.go's responseWith, keyed "route" instead of
// "pick".
func routeResponseWith(t *testing.T, choice string, confidence float64) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"model": "jev-1.13.0",
		"answers": map[string]any{
			"route": map[string]any{
				"type":          "choice",
				"choice":        choice,
				"confidence":    confidence,
				"probabilities": map[string]float64{choice: confidence},
			},
		},
		"usage": map[string]any{"input_tokens": 500, "output_tokens": 2},
	})
	require.NoError(t, err)

	return string(body)
}

// TestServiceRouterRequestShapeIsOneQuestionOnePerServicePlusCatchAll is
// this round's own "request body exact": shortlistCatalog's two services
// (inventory, attendance) each get one criterion, plus routeOtherID
// ("other") - and, critically, none of the three built-ins
// (list_capabilities, propose_panel, none) buildRequest's own criteriaFor
// always appends for the pick question.
func TestServiceRouterRequestShapeIsOneQuestionOnePerServicePlusCatchAll(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(routeResponseWith(t, "inventory", 0.9))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	router := jev.NewServiceRouter(server.URL, "test-key", nil)

	_, err := router.Route(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, questions, 1, "exactly one question")

	route, ok := questions["route"].(map[string]any)
	require.True(t, ok, "the one question must be named \"route\"")
	assert.Equal(t, "choice", route["type"])

	criteria, ok := route["criteria"].(map[string]any)
	require.True(t, ok)

	assert.ElementsMatch(t, []string{"inventory", "attendance", "other"}, keysOf(criteria),
		"one option per service plus the catch-all, and nothing else")
}

// TestServiceRouterCriteriaV2RequestShapeMatchesV1 proves
// WithRouterCriteria(jev.CriteriaV2) sends the same option keys as the
// CriteriaV1 default, just as object-valued criteria instead of plain
// strings.
func TestServiceRouterCriteriaV2RequestShapeMatchesV1(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(routeResponseWith(t, "inventory", 0.9))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	router := jev.NewServiceRouter(server.URL, "test-key", nil, jev.WithRouterCriteria(jev.CriteriaV2))

	_, err := router.Route(context.Background(), "在庫を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	route, ok := questions["route"].(map[string]any)
	require.True(t, ok)

	criteria, ok := route["criteria"].(map[string]any)
	require.True(t, ok)

	assert.ElementsMatch(t, []string{"inventory", "attendance", "other"}, keysOf(criteria))

	other, ok := criteria["other"].(map[string]any)
	require.True(t, ok, "CriteriaV2 sends each option as an object, including the catch-all")
	assert.NotEmpty(t, other["what"])
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

// TestServiceRouterConfidentAnswerNamesTheService proves a plain answer
// (not the catch-all) is carried straight through as ServiceRoute.
func TestServiceRouterConfidentAnswerNamesTheService(t *testing.T) {
	server := stubServer(t, http.StatusOK, routeResponseWith(t, "attendance", 0.92))
	router := jev.NewServiceRouter(server.URL, "test-key", nil)

	route, err := router.Route(context.Background(), "勤怠を見せて", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.Equal(t, "attendance", route.Service)
	assert.InDelta(t, 0.92, route.Confidence, 0.0001)
}

// TestServiceRouterCatchAllIsNoOpinion proves the catch-all option
// ("other") winning maps to ServiceRoute{} - empty Service, the "no
// opinion" internal/usecase/orchestrator.go's routeService treats as a
// fail-open case.
func TestServiceRouterCatchAllIsNoOpinion(t *testing.T) {
	server := stubServer(t, http.StatusOK, routeResponseWith(t, "other", 0.99))
	router := jev.NewServiceRouter(server.URL, "test-key", nil)

	route, err := router.Route(context.Background(), "今日の天気は？", nil, nil, shortlistCatalog())
	require.NoError(t, err)
	assert.Empty(t, route.Service)
}

// TestServiceRouterEmptyCatalogIsNoOpinionWithoutACall mirrors
// TestGateEmptyShortlistIsNotImpossibleWithoutACall: nothing to route
// against, so Route never calls the API at all.
func TestServiceRouterEmptyCatalogIsNoOpinionWithoutACall(t *testing.T) {
	called := false

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(server.Close)

	router := jev.NewServiceRouter(server.URL, "test-key", nil)

	route, err := router.Route(context.Background(), "q", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Empty(t, route.Service)
	assert.False(t, called)
}

// TestServiceRouterNonTwoHundredIsAnError proves a failing response
// reaches the caller as an error - Route never fails open on its own;
// that is internal/usecase/orchestrator.go's routeService job.
func TestServiceRouterNonTwoHundredIsAnError(t *testing.T) {
	server := stubServer(t, http.StatusInternalServerError, `{"error":"boom"}`)
	router := jev.NewServiceRouter(server.URL, "test-key", nil)

	_, err := router.Route(context.Background(), "q", nil, nil, shortlistCatalog())
	require.Error(t, err)
}

// TestServiceRouterMissingAnswerKeyIsAnError proves a 200 response that
// never names the "route" answer key still surfaces as an error, rather
// than a silently zero-valued route.
func TestServiceRouterMissingAnswerKeyIsAnError(t *testing.T) {
	server := stubServer(t, http.StatusOK, `{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`)
	router := jev.NewServiceRouter(server.URL, "test-key", nil)

	_, err := router.Route(context.Background(), "q", nil, nil, shortlistCatalog())
	require.Error(t, err)
}
