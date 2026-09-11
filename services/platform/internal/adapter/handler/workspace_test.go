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
