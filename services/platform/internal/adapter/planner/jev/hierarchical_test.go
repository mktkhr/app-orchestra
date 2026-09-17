package jev_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// hierarchical_test.go: buildHierarchicalRequest and Picker.Pick's own
// switch into it (needsHierarchical(shortlist), picker.go) - the >255
// option case this package's flat "pick" question (mapping.go's
// buildRequest) cannot carry as one Jev "choice" question at all
// (docs.typesafe.ai/primitives/choice's own 255-option ceiling). The <=255
// case (picker_test.go's own tests, plus TestPick20EndpointRequestIsByte...
// below) must stay byte-identical to buildRequest's own output - this file
// never touches that path, only asserts it.

// bigCatalog returns a catalogue of serviceCount services, each with
// opsPerService endpoints, named deterministically (svcN / opN_M) - large
// enough to exercise needsHierarchical's own >255 threshold without
// relying on any real fixture.
func bigCatalog(serviceCount, opsPerService int) domain.Catalog {
	endpoints := make([]domain.Endpoint, 0, serviceCount*opsPerService)

	for s := range serviceCount {
		svc := fmt.Sprintf("svc%d", s)

		for o := range opsPerService {
			endpoints = append(endpoints, domain.Endpoint{
				Service:            svc,
				OperationID:        fmt.Sprintf("op%d_%d", s, o),
				ServiceDisplayName: fmt.Sprintf("サービス%d", s),
				Summary:            fmt.Sprintf("操作%d-%d の一覧を返す", s, o),
				Method:             domain.MethodGet,
				Path:               fmt.Sprintf("/svc%d/op%d", s, o),
				Response:           &domain.Schema{Type: domain.SchemaTypeObject},
			})
		}
	}

	return domain.Catalog{Endpoints: endpoints}
}

// hierarchicalResponseBody builds a wireResponse JSON body naming a
// "service" answer and one "op_<service>" answer per entry of opChoices,
// each carrying probabilities for every id in probs.
func hierarchicalResponseBody(
	t *testing.T, serviceChoice string, serviceProbs map[string]float64,
	opAnswers map[string]map[string]float64,
) string {
	t.Helper()

	answers := map[string]any{
		"service": map[string]any{
			"type": "choice", "choice": serviceChoice, "confidence": serviceProbs[serviceChoice],
			"probabilities": serviceProbs,
		},
	}

	for svc, probs := range opAnswers {
		best := ""
		bestP := -1.0

		for id, p := range probs {
			if p > bestP {
				best, bestP = id, p
			}
		}

		answers["op_"+svc] = map[string]any{
			"type": "choice", "choice": best, "confidence": bestP, "probabilities": probs,
		}
	}

	body, err := json.Marshal(map[string]any{
		"model": "jev-1.13.0", "answers": answers,
		"usage": map[string]any{"input_tokens": 900, "output_tokens": 4},
	})
	require.NoError(t, err)

	return string(body)
}

func hierarchicalStubServer(t *testing.T, body string) *httptest.Server {
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

// capturedShapeRequest holds one requestShapeServer request's decoded body -
// a struct, not a bare *map[string]any, since gocritic's ptrToRefParam flags
// a pointer to an already-reference map type even as a return value (the
// same reasoning mapping_v5_test.go's own capturedRequest gives).
type capturedShapeRequest struct {
	body map[string]any
}

// requestShapeServer starts a server answering every request with a fixed
// three-service hierarchical response body (one svc0 candidate favoured),
// decoding each request it receives into the returned capturedShapeRequest
// - the shared setup TestPickHierarchicalRequestShape and
// TestPickHierarchicalWithNoWorkspaceOmitsProposePanelFromTheServiceQuestion
// both need, factored out so golangci's dupl check sees one copy, not two.
func requestShapeServer(t *testing.T) (*httptest.Server, *capturedShapeRequest) {
	t.Helper()

	captured := &capturedShapeRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured.body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		body := hierarchicalResponseBody(t, "svc0", map[string]float64{"svc0": 0.9, "svc1": 0.05, "svc2": 0.03},
			map[string]map[string]float64{"svc0": {"op0_0": 0.8}})
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, captured
}

// TestPickHierarchicalRequestShape asserts the >255 request's own shape:
// one "service" question naming every service present plus the three
// built-ins, one "op_<service>" question per service naming that service's
// own endpoints only - no built-ins.
func TestPickHierarchicalRequestShape(t *testing.T) {
	catalog := bigCatalog(3, 90) // 270 endpoints + 3 built-ins > 255

	server, captured := requestShapeServer(t)
	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	gotBody := captured.body

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	require.Len(t, questions, 4) // "service" + 3 "op_svcN"

	serviceQuestion, ok := questions["service"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "choice", serviceQuestion["type"])

	serviceCriteria, ok := serviceQuestion["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, serviceCriteria, 6) // 3 services + 3 built-ins
	assert.Contains(t, serviceCriteria, "svc0")
	assert.Contains(t, serviceCriteria, "svc1")
	assert.Contains(t, serviceCriteria, "svc2")
	assert.Contains(t, serviceCriteria, pick.IDListCapabilities)
	assert.Contains(t, serviceCriteria, pick.IDProposePanel)
	assert.Contains(t, serviceCriteria, pick.IDNone)

	for _, svc := range []string{"svc0", "svc1", "svc2"} {
		opQuestion, ok := questions["op_"+svc].(map[string]any)
		require.True(t, ok, "expected an op_%s question", svc)
		assert.Equal(t, "choice", opQuestion["type"])

		opCriteria, ok := opQuestion["criteria"].(map[string]any)
		require.True(t, ok)
		assert.Len(t, opCriteria, 90, "op_%s should carry only that service's own 90 endpoints", svc)
		assert.NotContains(t, opCriteria, pick.IDListCapabilities, "op_%s must carry no built-ins", svc)
		assert.NotContains(t, opCriteria, pick.IDProposePanel, "op_%s must carry no built-ins", svc)
		assert.NotContains(t, opCriteria, pick.IDNone, "op_%s must carry no built-ins", svc)
	}
}

// TestPickHierarchicalWithNoWorkspaceOmitsProposePanelFromTheServiceQuestion
// is O3 (docs/specs/offering.md) for the hierarchical path: the "service"
// question's own criteria must never carry propose_panel with no workspace
// - list_capabilities and none stay, unconditionally. Every "op_<service>"
// question already carries no built-ins at all regardless
// (TestPickHierarchicalRequestShape, above), so this test only needs to
// check "service".
func TestPickHierarchicalWithNoWorkspaceOmitsProposePanelFromTheServiceQuestion(t *testing.T) {
	catalog := bigCatalog(3, 90) // 270 endpoints + 3 built-ins > 255

	server, captured := requestShapeServer(t)
	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, catalog, usecase.PlanContext{})
	require.NoError(t, err)

	gotBody := captured.body

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	serviceQuestion, ok := questions["service"].(map[string]any)
	require.True(t, ok)

	serviceCriteria, ok := serviceQuestion["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, serviceCriteria, pick.IDListCapabilities)
	assert.NotContains(t, serviceCriteria, pick.IDProposePanel)
	assert.Contains(t, serviceCriteria, pick.IDNone)

	instructions, ok := serviceQuestion["instructions"].(string)
	require.True(t, ok)
	assert.NotContains(t, instructions, "propose_panel")
}

// TestPickHierarchicalCombinesServiceAndOpProbabilities: the winning
// endpoint is the one whose sqrt(P(service) * P(op|service)) is highest -
// not simply the endpoint with the single highest raw probability in
// isolation.
func TestPickHierarchicalCombinesServiceAndOpProbabilities(t *testing.T) {
	catalog := bigCatalog(2, 130) // 260 + 3 > 255

	// svc0 has the higher service probability (0.6) but its own best op is
	// weak (0.5) -> sqrt(0.6*0.5) ≈ 0.548.
	// svc1 has a lower service probability (0.4) but a near-certain op
	// (0.99) -> sqrt(0.4*0.99) ≈ 0.629, and should win.
	body := hierarchicalResponseBody(t, "svc0", map[string]float64{"svc0": 0.6, "svc1": 0.4},
		map[string]map[string]float64{
			"svc0": {"op0_0": 0.5, "op0_1": 0.3},
			"svc1": {"op1_0": 0.99, "op1_1": 0.01},
		})
	server := hierarchicalStubServer(t, body)
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "何かを見せて", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)
	assert.Equal(t, usecase.Pick{
		Kind: usecase.PickOperation, Service: "svc1", OperationID: "op1_0",
		Ambiguous: false, Confidence: result.Confidence,
	}, result)
	assert.InDelta(t, 0.6294, result.Confidence, 0.001)
}

// TestPickHierarchicalBuiltinCanWin: a built-in's own one-decision score
// (its own probability off the "service" answer) can outscore every
// two-decision endpoint path.
func TestPickHierarchicalBuiltinCanWin(t *testing.T) {
	catalog := bigCatalog(2, 130)

	body := hierarchicalResponseBody(t, pick.IDNone,
		map[string]float64{"svc0": 0.1, "svc1": 0.1, pick.IDNone: 0.75, pick.IDListCapabilities: 0.03, pick.IDProposePanel: 0.02},
		map[string]map[string]float64{
			"svc0": {"op0_0": 0.4},
			"svc1": {"op1_0": 0.4},
		})
	server := hierarchicalStubServer(t, body)
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "今日の天気は？", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)
	assert.Equal(t, usecase.PickNone, result.Kind)
	assert.Empty(t, result.OperationID)
	assert.InDelta(t, 0.75, result.Confidence, 0.0001)
	assert.False(t, result.Ambiguous)
}

// TestPickHierarchicalLowScoreIsAmbiguous mirrors S4 for the hierarchical
// path: a winning path score below the ambiguity threshold is Ambiguous.
func TestPickHierarchicalLowScoreIsAmbiguous(t *testing.T) {
	catalog := bigCatalog(2, 130)

	body := hierarchicalResponseBody(t, "svc0", map[string]float64{"svc0": 0.5, "svc1": 0.3},
		map[string]map[string]float64{
			"svc0": {"op0_0": 0.5},
			"svc1": {"op1_0": 0.5},
		})
	server := hierarchicalStubServer(t, body)
	picker := jev.New(server.URL, "test-key", nil)

	result, err := picker.Pick(context.Background(), "何かを見せて", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)
	// sqrt(0.5*0.5) = 0.5, at the default 0.5 threshold - not ambiguous;
	// confirm the boundary is treated the same way mapAnswer's own S4 rule
	// treats it (>= threshold is not ambiguous).
	assert.False(t, result.Ambiguous)

	picker = jev.New(server.URL, "test-key", nil, jev.WithAmbiguityThreshold(0.9))

	result, err = picker.Pick(context.Background(), "何かを見せて", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)
	assert.True(t, result.Ambiguous)
}

// TestPickHierarchicalServiceTooLargeErrors: a single service with more
// than 255 endpoints cannot be asked as one "op_<service>" choice question
// either - buildHierarchicalRequest returns ErrServiceTooLarge without ever
// calling the API.
func TestPickHierarchicalServiceTooLargeErrors(t *testing.T) {
	catalog := bigCatalog(1, 256) // one service alone over the 255 ceiling

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.Error(t, err)
	require.ErrorIs(t, err, jev.ErrServiceTooLarge)
	assert.False(t, called, "expected no call to the API when a service alone exceeds 255 endpoints")
}

// TestPick20EndpointRequestIsUnchanged: a 20-endpoint shortlist (this
// trial's own narrowing K=20, well under the 255 ceiling) must still go
// through buildRequest exactly as it always has - the hierarchical
// question shape (TestPickHierarchicalRequestShape, above) must never
// appear for it.
func TestPick20EndpointRequestIsUnchanged(t *testing.T) {
	catalog := bigCatalog(1, 20)

	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		body, err := json.Marshal(map[string]any{
			"model":   "jev-1.13.0",
			"answers": map[string]any{"pick": map[string]any{"type": "choice", "choice": "op0_0", "confidence": 0.9}},
			"usage":   map[string]any{"input_tokens": 700, "output_tokens": 3},
		})
		if err != nil {
			t.Errorf("marshaling fixture response: %v", err)
		}

		if _, err := w.Write(body); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "在庫を見せて", nil, nil, catalog, usecase.PlanContext{WorkspaceID: "ws-1"})
	require.NoError(t, err)

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	// Exactly the flat shape: one "pick" question, nothing named "service"
	// or "op_svc0".
	require.Len(t, questions, 1)
	assert.Contains(t, questions, "pick")

	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)

	criteria, ok := pickQuestion["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, criteria, 23) // 20 endpoints + 3 built-ins, exactly criteriaFor's own shape
}
