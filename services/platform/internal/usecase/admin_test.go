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

// adminFakeUserStore is a usecase.UserStore test double.
type adminFakeUserStore struct {
	users []domain.User
	err   error
}

func (f *adminFakeUserStore) List(context.Context) ([]domain.User, error) {
	return f.users, f.err
}

// adminFakePermissionStore is a usecase.PermissionStore test double.
type adminFakePermissionStore struct {
	byUser  map[string][]domain.Permission
	setErr  error
	forErr  error
	setCall struct {
		userID      string
		permissions []domain.Permission
	}
}

func (f *adminFakePermissionStore) For(_ context.Context, userID string) ([]domain.Permission, error) {
	if f.forErr != nil {
		return nil, f.forErr
	}

	return f.byUser[userID], nil
}

func (f *adminFakePermissionStore) Set(_ context.Context, userID string, permissions []domain.Permission) error {
	if f.setErr != nil {
		return f.setErr
	}

	f.setCall.userID = userID
	f.setCall.permissions = permissions

	return nil
}

var admin = &domain.User{ID: "usr-admin", Name: "admin", Role: domain.RoleAdmin}

var plainUser = &domain.User{ID: "usr-1", Name: "someone", Role: domain.RoleUser}

func TestAdminListUsersReturnsEveryAccountForAnAdmin(t *testing.T) {
	users := &adminFakeUserStore{users: []domain.User{
		{ID: "usr-admin", Name: "admin", Role: domain.RoleAdmin},
		{ID: "usr-1", Name: "someone", Role: domain.RoleUser},
	}}
	a := usecase.NewAdmin(users, &adminFakePermissionStore{})

	got, err := a.ListUsers(t.Context(), admin)

	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestAdminListUsersRefusesANonAdmin(t *testing.T) {
	a := usecase.NewAdmin(&adminFakeUserStore{}, &adminFakePermissionStore{})

	_, err := a.ListUsers(t.Context(), plainUser)

	require.Error(t, err)
	assert.ErrorIs(t, err, usecase.ErrNotAdmin)
}

func TestAdminListUsersRefusesANilUser(t *testing.T) {
	a := usecase.NewAdmin(&adminFakeUserStore{}, &adminFakePermissionStore{})

	_, err := a.ListUsers(t.Context(), nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, usecase.ErrNotAdmin)
}

func TestAdminListUsersWrapsAStoreError(t *testing.T) {
	boom := errors.New("boom")
	a := usecase.NewAdmin(&adminFakeUserStore{err: boom}, &adminFakePermissionStore{})

	_, err := a.ListUsers(t.Context(), admin)

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestAdminPermissionsReturnsWhatTheTargetHolds(t *testing.T) {
	permissions := &adminFakePermissionStore{byUser: map[string][]domain.Permission{
		"usr-1": {{Service: "inventory", OperationID: "ListInventoryItems"}},
	}}
	a := usecase.NewAdmin(&adminFakeUserStore{}, permissions)

	got, err := a.Permissions(t.Context(), admin, "usr-1")

	require.NoError(t, err)
	assert.Equal(t, []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}}, got)
}

func TestAdminPermissionsRefusesANonAdmin(t *testing.T) {
	a := usecase.NewAdmin(&adminFakeUserStore{}, &adminFakePermissionStore{})

	_, err := a.Permissions(t.Context(), plainUser, "usr-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, usecase.ErrNotAdmin)
}

func TestAdminPermissionsWrapsAStoreError(t *testing.T) {
	boom := errors.New("boom")
	a := usecase.NewAdmin(&adminFakeUserStore{}, &adminFakePermissionStore{forErr: boom})

	_, err := a.Permissions(t.Context(), admin, "usr-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestAdminSetPermissionsReplacesTheTargetsPermissions(t *testing.T) {
	permissions := &adminFakePermissionStore{}
	a := usecase.NewAdmin(&adminFakeUserStore{}, permissions)

	granted := []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}}
	err := a.SetPermissions(t.Context(), admin, "usr-1", granted)

	require.NoError(t, err)
	assert.Equal(t, "usr-1", permissions.setCall.userID)
	assert.Equal(t, granted, permissions.setCall.permissions)
}

func TestAdminSetPermissionsRefusesANonAdmin(t *testing.T) {
	permissions := &adminFakePermissionStore{}
	a := usecase.NewAdmin(&adminFakeUserStore{}, permissions)

	err := a.SetPermissions(t.Context(), plainUser, "usr-1", nil)

	require.ErrorIs(t, err, usecase.ErrNotAdmin)
	assert.Empty(t, permissions.setCall.userID)
}

func TestAdminSetPermissionsWrapsAStoreError(t *testing.T) {
	boom := errors.New("boom")
	a := usecase.NewAdmin(&adminFakeUserStore{}, &adminFakePermissionStore{setErr: boom})

	err := a.SetPermissions(t.Context(), admin, "usr-1", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}
