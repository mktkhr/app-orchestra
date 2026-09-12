package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	sqlitestore "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeWorkspaces is a test double for the workspaces interface workspace.go
// declares: it records its arguments and answers with whatever it was
// built with.
type fakeWorkspaces struct {
	listResult []domain.Workspace
	listErr    error

	getResult domain.Workspace
	getFound  bool
	getErr    error

	createResult domain.Workspace
	createErr    error
	createName   string

	deleteErr error
	deletedID string

	addPanelResult domain.Panel
	addPanelErr    error
	addPanelIn     *domain.Panel
	addPanelWSID   string

	deletePanelErr   error
	deletePanelWSID  string
	deletePanelPnlID string
}

func (f *fakeWorkspaces) List(context.Context, *domain.User) ([]domain.Workspace, error) {
	return f.listResult, f.listErr
}

func (f *fakeWorkspaces) Get(_ context.Context, _ *domain.User, _ string) (domain.Workspace, bool, error) {
	return f.getResult, f.getFound, f.getErr
}

func (f *fakeWorkspaces) Create(_ context.Context, _ *domain.User, name string) (domain.Workspace, error) {
	f.createName = name

	return f.createResult, f.createErr
}

func (f *fakeWorkspaces) Delete(_ context.Context, _ *domain.User, id string) error {
	f.deletedID = id

	return f.deleteErr
}

func (f *fakeWorkspaces) AddPanel(
	_ context.Context,
	_ *domain.User,
	workspaceID string,
	p *domain.Panel,
) (domain.Panel, error) {
	f.addPanelWSID = workspaceID
	f.addPanelIn = p

	return f.addPanelResult, f.addPanelErr
}

func (f *fakeWorkspaces) DeletePanel(_ context.Context, _ *domain.User, workspaceID, panelID string) error {
	f.deletePanelWSID = workspaceID
	f.deletePanelPnlID = panelID

	return f.deletePanelErr
}

func TestListWorkspacesRendersEachAsASummary(t *testing.T) {
	fake := &fakeWorkspaces{listResult: []domain.Workspace{
		{ID: "ws-1", Name: "在庫ボード", Panels: []domain.Panel{{ID: "pnl-1"}, {ID: "pnl-2"}}},
		{ID: "ws-2", Name: "空のボード"},
	}}

	h := handler.NewWorkspace(fake)

	resp, err := h.ListWorkspaces(t.Context(), openapi.ListWorkspacesRequestObject{})

	require.NoError(t, err)
	body, ok := resp.(openapi.ListWorkspaces200JSONResponse)
	require.True(t, ok)
	require.Len(t, body, 2)
	assert.Equal(t, "ws-1", body[0].Id)
	assert.Equal(t, "在庫ボード", body[0].Name)
	assert.Equal(t, 2, body[0].PanelCount)
	assert.Equal(t, 0, body[1].PanelCount)
}

func TestCreateWorkspacePassesTheNameThrough(t *testing.T) {
	fake := &fakeWorkspaces{createResult: domain.Workspace{ID: "ws-1", Name: "在庫ボード"}}

	h := handler.NewWorkspace(fake)

	resp, err := h.CreateWorkspace(t.Context(), openapi.CreateWorkspaceRequestObject{
		Body: &openapi.CreateWorkspaceRequest{Name: "在庫ボード"},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.CreateWorkspace201JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "ws-1", body.Id)
	assert.Equal(t, "在庫ボード", body.Name)
	assert.Equal(t, "在庫ボード", fake.createName)
}

func TestGetWorkspaceRendersItsPanels(t *testing.T) {
	fake := &fakeWorkspaces{
		getFound: true,
		getResult: domain.Workspace{
			ID:   "ws-1",
			Name: "在庫ボード",
			Panels: []domain.Panel{{
				ID: "pnl-1", WorkspaceID: "ws-1", Service: "inventory", OperationID: "ListInventoryItems",
				Component: "table", Title: "検品保留の在庫", Args: map[string]any{"status": "quarantined"}, Position: 0,
			}},
		},
	}

	h := handler.NewWorkspace(fake)

	resp, err := h.GetWorkspace(t.Context(), openapi.GetWorkspaceRequestObject{Id: "ws-1"})

	require.NoError(t, err)
	body, ok := resp.(openapi.GetWorkspace200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "ws-1", body.Id)
	require.Len(t, body.Panels, 1)
	assert.Equal(t, "pnl-1", body.Panels[0].Id)
	assert.Equal(t, openapi.Component("table"), body.Panels[0].Component)
	assert.Equal(t, "quarantined", body.Panels[0].Args["status"])
}

// TestGetWorkspaceRendersAPanelsView proves toAPIView's happy path: both
// halves of a domain.View convert to the wire View, and an empty
// Transform.Field (the count aggregate reads nothing) comes back absent,
// not present-but-blank (docs/plans/dashboard.md, Task 2).
func TestGetWorkspaceRendersAPanelsView(t *testing.T) {
	fake := &fakeWorkspaces{
		getFound: true,
		getResult: domain.Workspace{
			ID: "ws-1",
			Panels: []domain.Panel{{
				ID: "pnl-1", Service: "inventory", OperationID: "ListInventoryItems", Component: "chart",
				View: &domain.View{
					Transform: &domain.Transform{GroupBy: "status", Aggregate: domain.AggregateCount},
					Chart:     &domain.Chart{Category: "status", Value: "count", Kind: domain.ChartKindBar},
				},
			}},
		},
	}

	h := handler.NewWorkspace(fake)

	resp, err := h.GetWorkspace(t.Context(), openapi.GetWorkspaceRequestObject{Id: "ws-1"})

	require.NoError(t, err)
	body, ok := resp.(openapi.GetWorkspace200JSONResponse)
	require.True(t, ok)
	require.Len(t, body.Panels, 1)
	view := body.Panels[0].View
	require.NotNil(t, view)
	require.NotNil(t, view.Transform)
	assert.Equal(t, "status", view.Transform.GroupBy)
	assert.Equal(t, openapi.ViewTransformAggregate("count"), view.Transform.Aggregate)
	assert.Nil(t, view.Transform.Field, "an empty Field must come back absent, not a pointer to an empty string")
	require.NotNil(t, view.Chart)
	assert.Equal(t, "status", view.Chart.Category)
	assert.Equal(t, "count", view.Chart.Value)
	assert.Equal(t, openapi.ViewChartKind("bar"), view.Chart.Kind)
}

// TestGetWorkspaceRendersAPanelWithATransformOnlyView proves a View can
// carry just a Transform, with no Chart, and that a non-empty Field comes
// back as a present pointer (docs/specs/dashboard.md, section 3: the two
// halves are independent).
func TestGetWorkspaceRendersAPanelWithATransformOnlyView(t *testing.T) {
	fake := &fakeWorkspaces{
		getFound: true,
		getResult: domain.Workspace{
			ID: "ws-1",
			Panels: []domain.Panel{{
				ID: "pnl-1", Service: "inventory", OperationID: "ListInventoryItems", Component: "table",
				View: &domain.View{Transform: &domain.Transform{GroupBy: "status", Aggregate: domain.AggregateSum, Field: "amount"}},
			}},
		},
	}

	h := handler.NewWorkspace(fake)

	resp, err := h.GetWorkspace(t.Context(), openapi.GetWorkspaceRequestObject{Id: "ws-1"})

	require.NoError(t, err)
	body, ok := resp.(openapi.GetWorkspace200JSONResponse)
	require.True(t, ok)
	view := body.Panels[0].View
	require.NotNil(t, view)
	require.NotNil(t, view.Transform)
	require.NotNil(t, view.Transform.Field)
	assert.Equal(t, "amount", *view.Transform.Field)
	assert.Nil(t, view.Chart)
}

func TestGetWorkspaceReturns404WhenNotFound(t *testing.T) {
	fake := &fakeWorkspaces{getFound: false}

	h := handler.NewWorkspace(fake)

	resp, err := h.GetWorkspace(t.Context(), openapi.GetWorkspaceRequestObject{Id: "does-not-exist"})

	require.NoError(t, err)
	_, ok := resp.(openapi.GetWorkspace404JSONResponse)
	assert.True(t, ok)
}

func TestDeleteWorkspaceDelegates(t *testing.T) {
	fake := &fakeWorkspaces{}

	h := handler.NewWorkspace(fake)

	resp, err := h.DeleteWorkspace(t.Context(), openapi.DeleteWorkspaceRequestObject{Id: "ws-1"})

	require.NoError(t, err)
	_, ok := resp.(openapi.DeleteWorkspace204Response)
	assert.True(t, ok)
	assert.Equal(t, "ws-1", fake.deletedID)
}

func TestAddPanelStoresAndRendersThePanel(t *testing.T) {
	fake := &fakeWorkspaces{addPanelResult: domain.Panel{
		ID: "pnl-1", WorkspaceID: "ws-1", Service: "inventory", OperationID: "ListInventoryItems",
		Component: "table", Title: "検品保留の在庫", Args: map[string]any{"status": "quarantined"}, Position: 0,
	}}

	h := handler.NewWorkspace(fake)

	resp, err := h.AddPanel(t.Context(), openapi.AddPanelRequestObject{
		Id: "ws-1",
		Body: &openapi.CreatePanelRequest{
			Service: "inventory", OperationId: "ListInventoryItems",
			Args: map[string]any{"status": "quarantined"}, Component: "table", Title: "検品保留の在庫",
		},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.AddPanel201JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "pnl-1", body.Id)
	require.NotNil(t, fake.addPanelIn)
	assert.Equal(t, "inventory", fake.addPanelIn.Service)
	assert.Equal(t, "ws-1", fake.addPanelWSID)
}

// TestAddPanelPassesTheViewThrough proves toDomainView's happy path: both
// halves of a wire View, with Transform.Field present, convert into the
// domain.View the usecase receives.
func TestAddPanelPassesTheViewThrough(t *testing.T) {
	fake := &fakeWorkspaces{addPanelResult: domain.Panel{ID: "pnl-1"}}
	h := handler.NewWorkspace(fake)

	field := "amount"

	resp, err := h.AddPanel(t.Context(), openapi.AddPanelRequestObject{
		Id: "ws-1",
		Body: &openapi.CreatePanelRequest{
			Service: "inventory", OperationId: "ListInventoryItems", Component: "chart", Title: "金額合計",
			View: &openapi.View{
				Transform: &struct {
					Aggregate openapi.ViewTransformAggregate `json:"aggregate"`
					Field     *string                        `json:"field,omitempty"`
					GroupBy   string                         `json:"groupBy"`
				}{Aggregate: "sum", Field: &field, GroupBy: "status"},
				Chart: &struct {
					Category string                `json:"category"`
					Kind     openapi.ViewChartKind `json:"kind"`
					Value    string                `json:"value"`
				}{Category: "status", Kind: "line", Value: "amount"},
			},
		},
	})

	require.NoError(t, err)
	require.IsType(t, openapi.AddPanel201JSONResponse{}, resp)
	require.NotNil(t, fake.addPanelIn)
	require.NotNil(t, fake.addPanelIn.View)
	require.NotNil(t, fake.addPanelIn.View.Transform)
	assert.Equal(t, "status", fake.addPanelIn.View.Transform.GroupBy)
	assert.Equal(t, domain.AggregateSum, fake.addPanelIn.View.Transform.Aggregate)
	assert.Equal(t, "amount", fake.addPanelIn.View.Transform.Field)
	require.NotNil(t, fake.addPanelIn.View.Chart)
	assert.Equal(t, domain.ChartKindLine, fake.addPanelIn.View.Chart.Kind)
}

// TestAddPanelPassesATransformOnlyViewWithNoField proves a request naming
// only a Transform, with no Field (the count aggregate), converts with a
// nil Chart and an empty Field - not a nil pointer dereference.
func TestAddPanelPassesATransformOnlyViewWithNoField(t *testing.T) {
	fake := &fakeWorkspaces{addPanelResult: domain.Panel{ID: "pnl-1"}}
	h := handler.NewWorkspace(fake)

	resp, err := h.AddPanel(t.Context(), openapi.AddPanelRequestObject{
		Id: "ws-1",
		Body: &openapi.CreatePanelRequest{
			Service: "inventory", OperationId: "ListInventoryItems", Component: "table", Title: "ステータス別件数",
			View: &openapi.View{
				Transform: &struct {
					Aggregate openapi.ViewTransformAggregate `json:"aggregate"`
					Field     *string                        `json:"field,omitempty"`
					GroupBy   string                         `json:"groupBy"`
				}{Aggregate: "count", GroupBy: "status"},
			},
		},
	})

	require.NoError(t, err)
	require.IsType(t, openapi.AddPanel201JSONResponse{}, resp)
	require.NotNil(t, fake.addPanelIn)
	require.NotNil(t, fake.addPanelIn.View)
	require.NotNil(t, fake.addPanelIn.View.Transform)
	assert.Empty(t, fake.addPanelIn.View.Transform.Field)
	assert.Nil(t, fake.addPanelIn.View.Chart)
}

func TestAddPanelReturns400ForAnUnexposedOperation(t *testing.T) {
	fake := &fakeWorkspaces{addPanelErr: usecase.ErrEndpointNotFound}

	h := handler.NewWorkspace(fake)

	resp, err := h.AddPanel(t.Context(), openapi.AddPanelRequestObject{
		Id: "ws-1",
		Body: &openapi.CreatePanelRequest{
			Service: "inventory", OperationId: "GetInventorySpec",
			Args: map[string]any{}, Component: "table", Title: "だめなやつ",
		},
	})

	require.NoError(t, err)
	_, ok := resp.(openapi.AddPanel400JSONResponse)
	assert.True(t, ok)
}

func TestAddPanelReturns404ForAnUnknownWorkspace(t *testing.T) {
	fake := &fakeWorkspaces{addPanelErr: sqlitestore.ErrWorkspaceNotFound}

	h := handler.NewWorkspace(fake)

	resp, err := h.AddPanel(t.Context(), openapi.AddPanelRequestObject{
		Id: "does-not-exist",
		Body: &openapi.CreatePanelRequest{
			Service: "inventory", OperationId: "ListInventoryItems",
			Args: map[string]any{}, Component: "table", Title: "検品保留の在庫",
		},
	})

	require.NoError(t, err)
	_, ok := resp.(openapi.AddPanel404JSONResponse)
	assert.True(t, ok)
}

func TestAddPanelReturns500ForAnUnexpectedStoreError(t *testing.T) {
	fake := &fakeWorkspaces{addPanelErr: errors.New("disk on fire")}

	h := handler.NewWorkspace(fake)

	_, err := h.AddPanel(t.Context(), openapi.AddPanelRequestObject{
		Id: "ws-1",
		Body: &openapi.CreatePanelRequest{
			Service: "inventory", OperationId: "ListInventoryItems",
			Args: map[string]any{}, Component: "table", Title: "検品保留の在庫",
		},
	})

	require.Error(t, err)
}

func TestDeletePanelDelegates(t *testing.T) {
	fake := &fakeWorkspaces{}

	h := handler.NewWorkspace(fake)

	resp, err := h.DeletePanel(t.Context(), openapi.DeletePanelRequestObject{Id: "ws-1", PanelId: "pnl-1"})

	require.NoError(t, err)
	_, ok := resp.(openapi.DeletePanel204Response)
	assert.True(t, ok)
	assert.Equal(t, "ws-1", fake.deletePanelWSID)
	assert.Equal(t, "pnl-1", fake.deletePanelPnlID)
}
