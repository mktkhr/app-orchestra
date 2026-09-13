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
	updatePanelIn    *domain.PanelPatch
	updatePanelWSID  string
	updatePanelPnlID string
	updatePanelFound bool
	updatePanelOut   domain.Panel
	updatePanelErr   error
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

func (f *fakeWorkspaceStore) UpdatePanel(
	_ context.Context, workspaceID, panelID string, patch domain.PanelPatch,
) (domain.Panel, bool, error) {
	f.updatePanelWSID = workspaceID
	f.updatePanelPnlID = panelID
	f.updatePanelIn = &patch

	if f.updatePanelErr != nil {
		return domain.Panel{}, false, f.updatePanelErr
	}

	if !f.updatePanelFound {
		return domain.Panel{}, false, nil
	}

	return f.updatePanelOut, true, nil
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

// grantedPermissions returns a usecase.PermissionStore granting exactly
// the operation exposedCatalog() exposes, regardless of which user it is
// asked about - enough for every test in this file but AC-P-107's own to
// pass a permission check it is not otherwise testing.
func grantedPermissions() *fakePermissionStore {
	return &fakePermissionStore{permissions: []domain.Permission{
		{Service: "inventory", OperationID: "ListInventoryItems"},
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
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, err := w.List(t.Context(), owner())

	require.NoError(t, err)
	assert.Equal(t, "owner-1", store.listOwner)
}

func TestWorkspacesCreateScopesToTheUser(t *testing.T) {
	store := &fakeWorkspaceStore{}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	ws, err := w.Create(t.Context(), owner(), "在庫ボード")

	require.NoError(t, err)
	assert.Equal(t, "在庫ボード", ws.Name)
	assert.Equal(t, "owner-1", store.createOwner)
	assert.Equal(t, store.createOwner, ws.Owner)
}

func TestWorkspacesDeleteDelegatesToTheStoreForTheOwner(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	require.NoError(t, w.Delete(t.Context(), owner(), "ws-1"))
	assert.Equal(t, "ws-1", store.deletedID)
}

func TestWorkspacesDeleteIsANoOpForSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	require.NoError(t, w.Delete(t.Context(), stranger(), "ws-1"))
	assert.Empty(t, store.deletedID, "the store must never be asked to delete a workspace the caller does not own")
}

func TestWorkspacesGetReturnsTheOwnersWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	ws, ok, err := w.Get(t.Context(), owner(), "ws-1")

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "ws-1", ws.ID)
}

func TestWorkspacesGetIsNotFoundForSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, ok, err := w.Get(t.Context(), stranger(), "ws-1")

	require.NoError(t, err)
	assert.False(t, ok, "a workspace belonging to somebody else must be reported not found, not forbidden")
}

func TestWorkspacesGetIsNotFoundWhenTheStoreHasNothing(t *testing.T) {
	store := &fakeWorkspaceStore{getFound: false}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, ok, err := w.Get(t.Context(), owner(), "ws-missing")

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestWorkspacesAddPanelRejectsAnOperationTheCatalogueDoesNotExpose(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "GetInventorySpec"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Nil(t, store.addPanelIn, "the store must never be reached for an unexposed operation")
}

// TestWorkspacesAddPanelRejectsAnOperationTheUserMayNotCall proves
// AC-P-107: an operation the full catalogue exposes, but that the calling
// user holds no permission for, is refused the same way an operation that
// does not exist at all is - the same sentinel, so a 400 never tells
// anybody what exists (docs/specs/dashboard.md, AC-P-107; the same rule
// Orchestrator.Invoke already applies via catalogFor).
func TestWorkspacesAddPanelRejectsAnOperationTheUserMayNotCall(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	permissions := &fakePermissionStore{} // grants nothing
	w := usecase.NewWorkspaces(store, exposedCatalog(), permissions)

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Equal(t, "owner-1", permissions.userID, "AddPanel must consult the permission store for a non-admin user")
	assert.Nil(t, store.addPanelIn, "the store must never be reached for an operation the user may not call")
}

// TestWorkspacesAddPanelNeverConsultsThePermissionStoreForAnAdmin mirrors
// Orchestrator.catalogFor's own admin exception: an admin's access to
// every configured operation must never depend on a row in the permission
// table (docs/specs/auth.md, section 4).
func TestWorkspacesAddPanelNeverConsultsThePermissionStoreForAnAdmin(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "admin-1"}, getFound: true}
	permissions := &fakePermissionStore{} // grants nothing
	w := usecase.NewWorkspaces(store, exposedCatalog(), permissions)
	admin := &domain.User{ID: "admin-1", Role: domain.RoleAdmin}

	_, err := w.AddPanel(t.Context(), admin, "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.NoError(t, err)
	assert.Empty(t, permissions.userID, "an admin's access must never depend on a call to PermissionStore.For")
}

func TestWorkspacesAddPanelStoresAnExposedOperation(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

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
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

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
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

func TestWorkspacesAddPanelRejectsSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, err := w.AddPanel(t.Context(), stranger(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
	assert.Nil(t, store.addPanelIn, "the store must never be reached for a workspace the caller does not own")
}

func TestWorkspacesAddPanelRejectsAnUnknownWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getFound: false}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, err := w.AddPanel(t.Context(), owner(), "ws-missing", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
}

// panelWorkspace builds a workspace owned by "owner-1" holding one panel,
// "pnl-1", calling inventory/ListInventoryItems - the fixed operation
// UpdatePanel's tests check against exposedCatalog()/grantedPermissions().
func panelWorkspace() domain.Workspace {
	return domain.Workspace{
		ID:    "ws-1",
		Owner: "owner-1",
		Panels: []domain.Panel{
			{ID: "pnl-1", WorkspaceID: "ws-1", Service: "inventory", OperationID: "ListInventoryItems", Title: "元のタイトル"},
		},
	}
}

func TestWorkspacesUpdatePanelDelegatesThePatchToTheStore(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
		updatePanelOut:   domain.Panel{ID: "pnl-1", Title: "新しいタイトル"},
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"
	panel, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Title: &title})

	require.NoError(t, err)
	assert.Equal(t, "新しいタイトル", panel.Title)
	require.NotNil(t, store.updatePanelIn)
	assert.Equal(t, "新しいタイトル", *store.updatePanelIn.Title)
	assert.Nil(t, store.updatePanelIn.Args, "a patch that never named args must reach the store with Args nil")
	assert.Nil(t, store.updatePanelIn.Component, "a patch that never named component must reach the store with Component nil")
	assert.Nil(t, store.updatePanelIn.View, "a patch that never named the view must reach the store with View nil")
	assert.Equal(t, "ws-1", store.updatePanelWSID)
	assert.Equal(t, "pnl-1", store.updatePanelPnlID)
}

func TestWorkspacesUpdatePanelDefaultsAnEmptyTitleToTheOperationID(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	empty := ""
	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Title: &empty})

	require.NoError(t, err)
	require.NotNil(t, store.updatePanelIn.Title)
	assert.Equal(t, "ListInventoryItems", *store.updatePanelIn.Title)
}

// TestWorkspacesUpdatePanelRejectsAnOperationTheUserMayNotCall is
// AC-P-109's own case for the panel's own (fixed) operation, re-checked
// against the caller's current permissions rather than only at AddPanel's
// own save time.
func TestWorkspacesUpdatePanelRejectsAnOperationTheUserMayNotCall(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: panelWorkspace(), getFound: true}
	permissions := &fakePermissionStore{} // grants nothing
	w := usecase.NewWorkspaces(store, exposedCatalog(), permissions)

	title := "新しいタイトル"
	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Title: &title})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrEndpointNotFound)
	assert.Nil(t, store.updatePanelIn, "the store must never be reached for an operation the user may not call")
}

func TestWorkspacesUpdatePanelRejectsSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: panelWorkspace(), getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"
	_, err := w.UpdatePanel(t.Context(), stranger(), "ws-1", "pnl-1", domain.PanelPatch{Title: &title})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
	assert.Nil(t, store.updatePanelIn, "the store must never be reached for a workspace the caller does not own")
}

func TestWorkspacesUpdatePanelRejectsAnUnknownWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getFound: false}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"
	_, err := w.UpdatePanel(t.Context(), owner(), "ws-missing", "pnl-1", domain.PanelPatch{Title: &title})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
}

// TestWorkspacesUpdatePanelRejectsAnUnknownPanel proves a panel id absent
// from the (owned) workspace's own panels is refused the same way - the
// same sentinel, so a 404 never tells a caller whether the workspace or the
// panel is the part that does not exist (AC-P-109).
func TestWorkspacesUpdatePanelRejectsAnUnknownPanel(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: panelWorkspace(), getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"
	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-missing", domain.PanelPatch{Title: &title})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
	assert.Nil(t, store.updatePanelIn, "the store must never be reached for an unknown panel id")
}

func TestWorkspacesUpdatePanelReportsStoreNotFound(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: false,
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"
	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Title: &title})

	require.Error(t, err)
	require.ErrorIs(t, err, usecase.ErrWorkspaceNotFound)
}

func TestWorkspacesUpdatePanelPropagatesAStoreError(t *testing.T) {
	sentinel := errors.New("boom")
	store := &fakeWorkspaceStore{
		getResult:      panelWorkspace(),
		getFound:       true,
		updatePanelErr: sentinel,
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"
	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Title: &title})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

func TestWorkspacesDeletePanelDelegatesToTheStoreForTheOwner(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	require.NoError(t, w.DeletePanel(t.Context(), owner(), "ws-1", "pnl-1"))
	assert.Equal(t, "ws-1", store.deletePanelWSID)
	assert.Equal(t, "pnl-1", store.deletePanelPnlID)
}

func TestWorkspacesDeletePanelIsANoOpForSomebodyElsesWorkspace(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	require.NoError(t, w.DeletePanel(t.Context(), stranger(), "ws-1", "pnl-1"))
	assert.Empty(t, store.deletePanelWSID, "the store must never be asked to delete a panel from a workspace the caller does not own")
}

// TestWorkspacesAddPanelDefaultsAnUnsetSizeToTheDomainDefault is
// docs/plans/layout.md Task 0's own case: a panel built with no Width or
// Height at all - the zero value a caller that said nothing about size
// leaves - reaches the store already at domain.DefaultPanelWidth/
// DefaultPanelHeight (docs/specs/layout.md, section 3), not zero.
func TestWorkspacesAddPanelDefaultsAnUnsetSizeToTheDomainDefault(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})

	require.NoError(t, err)
	require.NotNil(t, store.addPanelIn)
	assert.Equal(t, domain.DefaultPanelWidth, store.addPanelIn.Width)
	assert.Equal(t, domain.DefaultPanelHeight, store.addPanelIn.Height)
	assert.Nil(t, store.addPanelIn.NarrowHeight,
		"a panel with no narrow height of its own must reach the store with a nil NarrowHeight, "+
			"not a default the way Width/Height get one (docs/specs/layout.md, section 5a, AC-L-108)")
}

// TestWorkspacesAddPanelClampsAnOutOfRangeSize is the same case for a
// caller that did send a width and height, but ones the grid cannot draw -
// 40 columns, and 0 rows. Clamping happens here, in the usecase, never in
// the browser (docs/plans/layout.md, Task 0).
func TestWorkspacesAddPanelClampsAnOutOfRangeSize(t *testing.T) {
	store := &fakeWorkspaceStore{getResult: domain.Workspace{ID: "ws-1", Owner: "owner-1"}, getFound: true}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	zeroNarrowHeight := 0

	_, err := w.AddPanel(t.Context(), owner(), "ws-1", &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Width: 40, Height: 0, NarrowHeight: &zeroNarrowHeight,
	})

	require.NoError(t, err)
	require.NotNil(t, store.addPanelIn)
	assert.Equal(t, domain.MaxPanelWidth, store.addPanelIn.Width, "a width of 40 must clamp down to the maximum")
	assert.Equal(t, domain.MinPanelHeight, store.addPanelIn.Height, "a height of 0 must clamp up to the minimum")
	require.NotNil(t, store.addPanelIn.NarrowHeight)
	assert.Equal(t, domain.MinPanelHeight, *store.addPanelIn.NarrowHeight,
		"a narrow height of 0 must clamp up to the minimum, the same as height")
}

// TestWorkspacesUpdatePanelClampsAWidthTheCallerNamed proves UpdatePanel
// clamps a width or height a PATCH actually named, the same way AddPanel
// clamps one supplied at create time - a caller that is not this
// repository's own frontend can send -1 here just as easily.
func TestWorkspacesUpdatePanelClampsAWidthTheCallerNamed(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
		updatePanelOut:   domain.Panel{ID: "pnl-1", Width: domain.MinPanelWidth},
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	width := -1

	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Width: &width})

	require.NoError(t, err)
	require.NotNil(t, store.updatePanelIn)
	require.NotNil(t, store.updatePanelIn.Width)
	assert.Equal(t, domain.MinPanelWidth, *store.updatePanelIn.Width, "a width of -1 must clamp up to the minimum")
}

// TestWorkspacesUpdatePanelLeavesSizeUnnamedFieldsNil proves a PATCH that
// never names Width, Height, NarrowHeight or Position reaches the store
// with all four nil - absent-means-unchanged, the same as
// Args/Component/View (docs/specs/dashboard.md, P11; AC-P-108) - so the
// store never overwrites a column the caller said nothing about.
func TestWorkspacesUpdatePanelLeavesSizeUnnamedFieldsNil(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
		updatePanelOut:   domain.Panel{ID: "pnl-1"},
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	title := "新しいタイトル"

	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Title: &title})

	require.NoError(t, err)
	require.NotNil(t, store.updatePanelIn)
	assert.Nil(t, store.updatePanelIn.Width)
	assert.Nil(t, store.updatePanelIn.Height)
	assert.Nil(t, store.updatePanelIn.NarrowHeight)
	assert.Nil(t, store.updatePanelIn.Position)
}

// TestWorkspacesUpdatePanelClampsNarrowHeightTheCallerNamed is
// TestWorkspacesUpdatePanelClampsAWidthTheCallerNamed's own case for
// NarrowHeight - a caller that is not this repository's own frontend can
// send 0 here just as easily as an out-of-range width (docs/specs/layout.md,
// section 5a).
func TestWorkspacesUpdatePanelClampsNarrowHeightTheCallerNamed(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
		updatePanelOut:   domain.Panel{ID: "pnl-1", NarrowHeight: new(domain.MinPanelHeight)},
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	narrowHeight := 0

	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{NarrowHeight: &narrowHeight})

	require.NoError(t, err)
	require.NotNil(t, store.updatePanelIn)
	require.NotNil(t, store.updatePanelIn.NarrowHeight)
	assert.Equal(t, domain.MinPanelHeight, *store.updatePanelIn.NarrowHeight,
		"a narrow height of 0 must clamp up to the minimum")
}

// TestWorkspacesUpdatePanelNarrowHeightLeavesHeightAlone is AC-L-107 at the
// usecase level: a PATCH naming only NarrowHeight reaches the store with
// Height nil - unchanged - proving the wide breakpoint's own height is
// untouched by a narrow-height edit (docs/specs/layout.md, section 5a).
func TestWorkspacesUpdatePanelNarrowHeightLeavesHeightAlone(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
		updatePanelOut:   domain.Panel{ID: "pnl-1", Height: 3, NarrowHeight: new(1)},
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	narrowHeight := 1

	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{NarrowHeight: &narrowHeight})

	require.NoError(t, err)
	require.NotNil(t, store.updatePanelIn)
	require.NotNil(t, store.updatePanelIn.NarrowHeight)
	assert.Equal(t, 1, *store.updatePanelIn.NarrowHeight)
	assert.Nil(t, store.updatePanelIn.Height, "changing NarrowHeight must leave Height nil - unchanged")
}

// TestWorkspacesUpdatePanelPassesPositionThrough proves a PATCH naming a
// new position reaches the store unchanged - reordering itself is not this
// task's job, only accepting the field is (docs/plans/layout.md, Task 0).
func TestWorkspacesUpdatePanelPassesPositionThrough(t *testing.T) {
	store := &fakeWorkspaceStore{
		getResult:        panelWorkspace(),
		getFound:         true,
		updatePanelFound: true,
		updatePanelOut:   domain.Panel{ID: "pnl-1", Position: 3},
	}
	w := usecase.NewWorkspaces(store, exposedCatalog(), grantedPermissions())

	position := 3

	_, err := w.UpdatePanel(t.Context(), owner(), "ws-1", "pnl-1", domain.PanelPatch{Position: &position})

	require.NoError(t, err)
	require.NotNil(t, store.updatePanelIn)
	require.NotNil(t, store.updatePanelIn.Position)
	assert.Equal(t, 3, *store.updatePanelIn.Position)
}
