package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeWorkspaceStore is an in-memory usecase.WorkspaceStore, capturing what
// each call was made with so tests can assert the owner used - a real
// user's ID now, not the placeholder every call used to be pinned to.
type fakeWorkspaceStore struct {
	listOwner        string
	listResult       []domain.Workspace
	getResult        domain.Workspace
	getFound         bool
	createOwner      string
	createName       string
	createErr        error
	addPanelErr      error
	addPanelIn       *domain.Panel
	deletedID        string
	deletePanelWSID  string
	deletePanelPnlID string
}

func (f *fakeWorkspaceStore) List(_ context.Context, owner string) ([]domain.Workspace, error) {
	f.listOwner = owner

	return f.listResult, nil
}

func (f *fakeWorkspaceStore) Get(_ context.Context, _ string) (domain.Workspace, bool, error) {
	return f.getResult, f.getFound, nil
}

func (f *fakeWorkspaceStore) Create(_ context.Context, owner, name string) (domain.Workspace, error) {
	f.createOwner = owner
	f.createName = name

	if f.createErr != nil {
		return domain.Workspace{}, f.createErr
	}

	return domain.Workspace{ID: "ws-1", Owner: owner, Name: name}, nil
}

func (f *fakeWorkspaceStore) Delete(_ context.Context, id string) error {
	f.deletedID = id

	return nil
}

func (f *fakeWorkspaceStore) AddPanel(_ context.Context, _ string, p *domain.Panel) (domain.Panel, error) {
	f.addPanelIn = p
	if f.addPanelErr != nil {
		return domain.Panel{}, f.addPanelErr
	}

	stored := *p
	stored.ID = "pnl-1"

	return stored, nil
}

func (f *fakeWorkspaceStore) DeletePanel(_ context.Context, workspaceID, panelID string) error {
	f.deletePanelWSID = workspaceID
	f.deletePanelPnlID = panelID

	return nil
}

// exposedCatalog builds a one-endpoint catalogue whose only endpoint is
// inventory/ListInventoryItems, exposed - enough to distinguish "exposed"
// from "not exposed" in AddPanel's tests without a real service.
func exposedCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{Service: "inventory", OperationID: "ListInventoryItems", Method: domain.MethodGet},
	}}
}

// owner is the user every "happy path" test in this file acts as: a
// workspace it reads or writes belongs to owner.ID.
func owner() *domain.User {
	return &domain.User{ID: "owner-1", Role: domain.RoleUser}
}

// stranger is a different user, used by the tests proving a workspace
// belonging to somebody else is not found rather than forbidden
// (docs/specs/auth.md, section 7).
func stranger() *domain.User {
	return &domain.User{ID: "stranger-1", Role: domain.RoleUser}
}

func TestWorkspacesListScopesToTheUser(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.List(t.Context(), owner())

	require.NoError(t, err)
	assert.Equal(t, "owner-1", store.listOwner)
}

func TestWorkspacesCreateScopesToTheUser(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	ws, err := w.Create(t.Context(), owner(), "在庫ボード")

	require.NoError(t, err)
	assert.Equal(t, "在庫ボード", ws.Name)
	assert.Equal(t, "owner-1", store.createOwner)
	assert.Equal(t, store.createOwner, ws.Owner)
}

func TestWorkspacesDeleteDelegatesToTheStoreForTheOwner(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	require.NoError(t, w.Delete(t.Context(), owner(), "ws-1"))
	assert.Equal(t, "ws-1", store.deletedID)
}

func TestWorkspacesDeleteIsANoOpForSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	require.NoError(t, w.Delete(t.Context(), stranger(), "ws-1"))
	assert.Empty(t, store.deletedID, "the store must never be asked to delete a workspace the caller does not own")
}

func TestWorkspacesGetReturnsTheOwnersWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	ws, ok, err := w.Get(t.Context(), owner(), "ws-1")

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "ws-1", ws.ID)
}

func TestWorkspacesGetIsNotFoundForSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, ok, err := w.Get(t.Context(), stranger(), "ws-1")

	require.NoError(t, err)
	assert.False(t, ok, "a workspace belonging to somebody else must be reported not found, not forbidden")
}

func TestWorkspacesGetIsNotFoundWhenTheStoreHasNothing(t *testing.T) {
	store := &fakeWorkspaceStore{getFound: false}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, ok, err := w.Get(t.Context(), owner(), "ws-missing")

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestWorkspacesAddPanelRejectsAnOperationTheCatalogueDoesNotExpose(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "GetInventorySpec"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Nil(t, store.addPanelIn, "the store must never be reached for an unexposed operation")
}

func TestWorkspacesAddPanelStoresAnExposedOperation(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	panel, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Title: "検品保留の在庫",
	})

	require.NoError(t, err)
	assert.Equal(t, "pnl-1", panel.ID)
	require.NotNil(t, store.addPanelIn)
	assert.Equal(t, "検品保留の在庫", store.addPanelIn.Title)
}

func TestWorkspacesAddPanelDefaultsAnEmptyTitleToTheOperationID(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.NoError(t, err)
	require.NotNil(t, store.addPanelIn)
	assert.Equal(t, "ListInventoryItems", store.addPanelIn.Title)
}

func TestWorkspacesAddPanelPropagatesAStoreError(t *testing.T) {
	sentinel := errors.New("boom")
	store := &fakeWorkspaceStore{
		getResult:   domain.Workspace{ID: "ws-1", Owner: "owner-1"},
		getFound:    true,
		addPanelErr: sentinel,
	}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

func TestWorkspacesAddPanelRejectsSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), stranger(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
	assert.Nil(t, store.addPanelIn, "the store must never be reached for a workspace the caller does not own")
}

func TestWorkspacesAddPanelRejectsAnUnknownWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getFound: false}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), owner(), "ws-missing", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
}

func TestWorkspacesDeletePanelDelegatesToTheStoreForTheOwner(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	require.NoError(t, w.DeletePanel(t.Context(), owner(), "ws-1", "pnl-1"))
	assert.Equal(t, "ws-1", store.deletePanelWSID)
	assert.Equal(t, "pnl-1", store.deletePanelPnlID)
}

func TestWorkspacesDeletePanelIsANoOpForSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	require.NoError(t, w.DeletePanel(t.Context(), stranger(), "ws-1", "pnl-1"))
	assert.Empty(t, store.deletePanelWSID, "the store must never be asked to delete a panel from a workspace the caller does not own")
}
