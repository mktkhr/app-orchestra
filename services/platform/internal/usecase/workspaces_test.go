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
// each call was made with so tests can assert the stub owner was used
// without a real database.
type fakeWorkspaceStore struct {
	listOwner   string
	listResult  []domain.Workspace
	getResult   domain.Workspace
	getFound    bool
	createOwner string
	createName  string
	createErr   error
	addPanelErr error
	addPanelIn  *domain.Panel
	deletedID   string
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

func (f *fakeWorkspaceStore) DeletePanel(_ context.Context, _, _ string) error {
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

func TestWorkspacesListUsesTheStubOwner(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.List(t.Context())

	require.NoError(t, err)
	assert.NotEmpty(t, store.listOwner, "List must scope the store call to an owner")
}

func TestWorkspacesCreateUsesTheStubOwner(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	ws, err := w.Create(t.Context(), "在庫ボード")

	require.NoError(t, err)
	assert.Equal(t, "在庫ボード", ws.Name)
	assert.Equal(t, store.createOwner, ws.Owner)
	assert.NotEmpty(t, store.createOwner)
}

func TestWorkspacesDeleteDelegatesToTheStore(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	require.NoError(t, w.Delete(t.Context(), "ws-1"))
	assert.Equal(t, "ws-1", store.deletedID)
}

func TestWorkspacesGetDelegatesToTheStore(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	ws, ok, err := w.Get(t.Context(), "ws-1")

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "ws-1", ws.ID)
}

func TestWorkspacesAddPanelRejectsAnOperationTheCatalogueDoesNotExpose(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "GetInventorySpec"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Nil(t, store.addPanelIn, "the store must never be reached for an unexposed operation")
}

func TestWorkspacesAddPanelStoresAnExposedOperation(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	panel, err := w.AddPanel(t.Context(), "ws-1", &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Title: "検品保留の在庫",
	})

	require.NoError(t, err)
	assert.Equal(t, "pnl-1", panel.ID)
	require.NotNil(t, store.addPanelIn)
	assert.Equal(t, "検品保留の在庫", store.addPanelIn.Title)
}

func TestWorkspacesAddPanelDefaultsAnEmptyTitleToTheOperationID(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.NoError(t, err)
	require.NotNil(t, store.addPanelIn)
	assert.Equal(t, "ListInventoryItems", store.addPanelIn.Title)
}

func TestWorkspacesAddPanelPropagatesAStoreError(t *testing.T) {
	sentinel := errors.New("boom")
	store := &fakeWorkspaceStore{addPanelErr: sentinel}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	_, err := w.AddPanel(t.Context(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

func TestWorkspacesDeletePanelDelegatesToTheStore(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog())

	assert.NoError(t, w.DeletePanel(t.Context(), "ws-1", "pnl-1"))
}
