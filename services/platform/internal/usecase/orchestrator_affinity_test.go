package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// affinityCatalog is a two-service catalogue shaped like the real
// attendance/inventory pair (TODO.md, "real-attendance-detail"): each
// service has one get-by-id endpoint whose id parameter declares the same
// kind of Pattern the real contracts do, plus one endpoint with no
// Pattern at all, so a question naming no id still has candidates to
// leave unnarrowed.
func affinityCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service: "attendance", OperationID: "GetAttendanceRecord", Method: domain.MethodGet,
			Path: "/api/attendance/records/{id}", Summary: "Get one attendance record by id.",
			DisplayName: "勤怠記録の詳細",
			Parameters: []domain.Parameter{
				{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString, Pattern: "^att-[0-9]+$"}},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service: "attendance", OperationID: "ListAttendanceRecords", Method: domain.MethodGet,
			Path: "/api/attendance/records", Summary: "List attendance records.",
			DisplayName: "勤怠記録の一覧", Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service: "inventory", OperationID: "GetInventoryItem", Method: domain.MethodGet,
			Path: "/api/inventory/items/{id}", Summary: "Get one stock item by id.",
			DisplayName: "在庫アイテムの詳細",
			Parameters: []domain.Parameter{
				{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString, Pattern: "^itm-[0-9]+$"}},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
		{
			Service: "inventory", OperationID: "ListInventoryItems", Method: domain.MethodGet,
			Path: "/api/inventory/items", Summary: "List stock items.",
			DisplayName: "在庫アイテムの一覧", Response: &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

// affinityOrchestrator builds a two-stage Orchestrator over
// affinityCatalog(), the same shape stagedOrchestrator builds over
// stagingShortlist (orchestrator_staging_test.go) - a pass-through
// narrower, and whatever picker the test supplies.
func affinityOrchestrator(t *testing.T, picker usecase.Picker, planner usecase.Planner, invoker usecase.Invoker) *usecase.Orchestrator {
	t.Helper()

	catalog := affinityCatalog()
	narrower := &fakeNarrower{catalog: catalog}

	return usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithNarrower(narrower, 20), usecase.WithPicker(picker), usecase.WithStages(2),
	)
}

// TestPlanStagedNarrowsPickCatalogToTheServiceAnIdBelongsTo is the fix
// itself (TODO.md, "real-attendance-detail"; DECISIONS.md, 2026-09-17,
// "real-catalogue eval"): a question carrying a token matching only one
// service's own id Pattern - 「att-002の内容」for attendance,
// 「itm-001の詳細」for inventory - must offer the pick that service's
// endpoints alone, not the whole two-service shortlist.
func TestPlanStagedNarrowsPickCatalogToTheServiceAnIdBelongsTo(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		service     string
		operationID string
	}{
		{name: "attendance", query: "att-002の内容", service: "attendance", operationID: "GetAttendanceRecord"},
		{name: "inventory", query: "itm-001の詳細", service: "inventory", operationID: "GetInventoryItem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickOperation, Service: tt.service, OperationID: tt.operationID}}
			planner := &fakePlanner{decision: usecase.Decision{Kind: usecase.DecisionCall, Service: tt.service, OperationID: tt.operationID}}
			invoker := &fakeInvoker{data: map[string]any{}}

			orchestrator := affinityOrchestrator(t, picker, planner, invoker)

			_, err := orchestrator.Plan(t.Context(), adminUser(), tt.query, nil, nil, "", "", nil)

			require.NoError(t, err)
			require.Equal(t, 1, picker.calls)

			services := make(map[string]struct{})
			for _, e := range picker.catalog.Endpoints {
				services[e.Service] = struct{}{}
			}

			assert.Equal(t, map[string]struct{}{tt.service: {}}, services,
				"the question's id matches only %s's own id Pattern, so the pick's shortlist must be narrowed to it", tt.service)
		})
	}
}

// TestPlanStagedLeavesPickCatalogUnchangedWithNoIdInTheQuestion is the
// no-match half: a question naming no id-shaped token must reach the pick
// with the shortlist exactly as planStaged received it.
func TestPlanStagedLeavesPickCatalogUnchangedWithNoIdInTheQuestion(t *testing.T) {
	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickListCapabilities}}
	planner := &fakePlanner{}
	invoker := &fakeInvoker{}

	orchestrator := affinityOrchestrator(t, picker, planner, invoker)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "何ができる？", nil, nil, "", "", nil)

	require.NoError(t, err)
	require.Equal(t, 1, picker.calls)
	assert.Equal(t, affinityCatalog(), picker.catalog,
		"no id-shaped token in the question - the pick's shortlist must be left exactly as planStaged received it")
}

// TestPlanStagedLeavesPickCatalogUnchangedWhenAnIdMatchesTwoServices covers
// the other unchanged case: an id shape shared by two services' own
// Patterns is not evidence for either one.
func TestPlanStagedLeavesPickCatalogUnchangedWhenAnIdMatchesTwoServices(t *testing.T) {
	catalog := affinityCatalog()
	// A pattern broad enough to match both att-002 and itm-001 -
	// deliberately wrong for this endpoint alone, so the test can put an
	// id shape in the question that both services now claim.
	catalog.Endpoints[3].Parameters = []domain.Parameter{
		{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString, Pattern: "^[a-z]+-[0-9]+$"}},
	}

	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickListCapabilities}}
	planner := &fakePlanner{}
	invoker := &fakeInvoker{}
	narrower := &fakeNarrower{catalog: catalog}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithNarrower(narrower, 20), usecase.WithPicker(picker), usecase.WithStages(2),
	)

	_, err := orchestrator.Plan(t.Context(), adminUser(), "att-002の内容", nil, nil, "", "", nil)

	require.NoError(t, err)
	require.Equal(t, 1, picker.calls)
	assert.Equal(t, catalog, picker.catalog,
		"att-002 now matches attendance's own pattern and inventory's widened one - two services, so unchanged")
}

// TestPlanStagedIgnoresAnInvalidPatternWithoutPanicking is the contract-
// quality half: a Pattern that fails to compile must be logged and
// ignored, never a panic, and must never itself count as a match.
func TestPlanStagedIgnoresAnInvalidPatternWithoutPanicking(t *testing.T) {
	catalog := affinityCatalog()
	catalog.Endpoints[0].Parameters[0].Schema.Pattern = "att-[0-9+" // unbalanced bracket: fails to compile

	picker := &fakePicker{pick: usecase.Pick{Kind: usecase.PickListCapabilities}}
	planner := &fakePlanner{}
	invoker := &fakeInvoker{}
	narrower := &fakeNarrower{catalog: catalog}
	orchestrator := usecase.NewOrchestrator(
		catalog, planner, invoker, &fakePermissionStore{},
		usecase.WithNarrower(narrower, 20), usecase.WithPicker(picker), usecase.WithStages(2),
	)

	buf := withCapturedDefaultLogger(t)

	require.NotPanics(t, func() {
		_, err := orchestrator.Plan(t.Context(), adminUser(), "att-002の内容", nil, nil, "", "", nil)
		require.NoError(t, err)
	})

	require.Equal(t, 1, picker.calls)
	assert.Equal(t, catalog, picker.catalog,
		"the only pattern in play fails to compile, so no service can be affirmed and the shortlist stays whole")
	assert.Contains(t, buf.String(), "invalid id pattern")
}
