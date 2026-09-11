package jsonmode_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jsonmode"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// liveTimeout bounds a call to a real, possibly slow local model. Generous
// for the same reason toolcall's own live test's is: this planner also
// retries once on a bad answer, which can double the wall-clock time a
// slow backend takes.
const liveTimeout = 90 * time.Second

// TestPlanAgainstALiveModel exercises this package's whole path - real
// HTTP, a real model answering with JSON instead of a tool call - against
// whatever OpenAI-compatible endpoint ORCHESTRA_LLM_BASE_URL names. It
// never runs under `make check` (the global constraint,
// docs/plans/orchestration.md): it t.Skip()s unless ORCHESTRA_LIVE_LLM=1,
// which nothing in the harness sets. Run it by hand against llama-swap,
// mirroring internal/adapter/planner/toolcall/live_test.go:
//
//	ORCHESTRA_LIVE_LLM=1 ORCHESTRA_LLM_BASE_URL=http://localhost:11435/v1 \
//	  ORCHESTRA_LLM_MODEL=qwen3.5-9b-q8 go test ./internal/adapter/planner/jsonmode/... -run Live -v
func TestPlanAgainstALiveModel(t *testing.T) {
	if os.Getenv("ORCHESTRA_LIVE_LLM") != "1" {
		t.Skip("set ORCHESTRA_LIVE_LLM=1 to run this test against a real model")
	}

	baseURL := os.Getenv("ORCHESTRA_LLM_BASE_URL")
	if baseURL == "" {
		t.Fatal("ORCHESTRA_LLM_BASE_URL must be set alongside ORCHESTRA_LIVE_LLM=1")
	}

	model := os.Getenv("ORCHESTRA_LLM_MODEL")

	catalog := domain.Catalog{
		Endpoints: []domain.Endpoint{
			{
				Service:     "inventory",
				OperationID: "ListInventoryItems",
				Method:      domain.MethodGet,
				Path:        "/api/inventory/items",
				Summary:     "List inventory items, optionally filtered by status.",
				Parameters: []domain.Parameter{
					{
						Name: "status",
						In:   "query",
						Schema: domain.Schema{
							Type:       domain.SchemaTypeString,
							Enum:       []string{"allocated", "staged", "quarantined", "consigned"},
							EnumLabels: map[string]string{"allocated": "引当済", "staged": "出荷準備完了", "quarantined": "検品保留", "consigned": "預託在庫"},
						},
					},
				},
				Response: &domain.Schema{
					Type: domain.SchemaTypeObject,
					Properties: map[string]domain.Schema{
						"items": {Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
					},
				},
			},
		},
	}

	client := chat.New(chat.Config{BaseURL: baseURL, APIKey: os.Getenv("ORCHESTRA_LLM_API_KEY"), Model: model})
	planner := jsonmode.New(client, catalog)

	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	decision, err := planner.Plan(ctx, "検品保留の在庫を見せて", nil, usecase.ToolsFor(catalog))
	require.NoError(t, err)
	require.Equal(t, usecase.DecisionCall, decision.Kind)
	require.Equal(t, "ListInventoryItems", decision.OperationID)
	require.Equal(t, "quarantined", decision.Args["status"])
}
