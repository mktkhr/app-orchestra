package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeAdmin is a test double for the admin interface users.go declares.
type fakeAdmin struct {
	listResult []domain.User
	listErr    error

	permissionsResult []domain.Permission
	permissionsErr    error
	permissionsTarget string

	setPermissionsErr    error
	setPermissionsTarget string
	setPermissionsIn     []domain.Permission

	operationsResult []domain.Endpoint
	operationsErr    error
}

func (f *fakeAdmin) ListUsers(context.Context, *domain.User) ([]domain.User, error) {
	return f.listResult, f.listErr
}

func (f *fakeAdmin) Permissions(_ context.Context, _ *domain.User, targetUserID string) ([]domain.Permission, error) {
	f.permissionsTarget = targetUserID

	return f.permissionsResult, f.permissionsErr
}

func (f *fakeAdmin) SetPermissions(
	_ context.Context,
	_ *domain.User,
	targetUserID string,
	permissions []domain.Permission,
) error {
	f.setPermissionsTarget = targetUserID
	f.setPermissionsIn = permissions

	return f.setPermissionsErr
}

func (f *fakeAdmin) Operations(context.Context, *domain.User) ([]domain.Endpoint, error) {
	return f.operationsResult, f.operationsErr
}

var testAdminUser = &domain.User{ID: "usr-admin", Name: "admin", Role: domain.RoleAdmin}

func TestUsersListUsersReturnsEveryAccount(t *testing.T) {
	admin := &fakeAdmin{listResult: []domain.User{
		{ID: "usr-admin", Name: "admin", Role: domain.RoleAdmin},
		{ID: "usr-1", Name: "someone", Role: domain.RoleUser},
	}}
	h := handler.NewUsers(admin)

	resp, err := h.ListUsers(handler.WithUser(t.Context(), testAdminUser), openapi.ListUsersRequestObject{})

	require.NoError(t, err)
	list, ok := resp.(openapi.ListUsers200JSONResponse)
	require.True(t, ok)
	require.Len(t, list, 2)
	assert.Equal(t, "admin", list[0].Name)
	assert.Equal(t, openapi.Role(domain.RoleUser), list[1].Role)
}

func TestUsersListUsersReturns403ForANonAdmin(t *testing.T) {
	admin := &fakeAdmin{listErr: usecase.ErrNotAdmin}
	h := handler.NewUsers(admin)

	resp, err := h.ListUsers(t.Context(), openapi.ListUsersRequestObject{})

	require.NoError(t, err)
	_, ok := resp.(openapi.ListUsers403JSONResponse)
	assert.True(t, ok)
}

func TestUsersListUsersReturnsAnErrorForAGenuineFailure(t *testing.T) {
	boom := errors.New("boom")
	h := handler.NewUsers(&fakeAdmin{listErr: boom})

	_, err := h.ListUsers(t.Context(), openapi.ListUsersRequestObject{})

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestUsersGetUserPermissionsReturnsWhatTheTargetHolds(t *testing.T) {
	admin := &fakeAdmin{permissionsResult: []domain.Permission{
		{Service: "inventory", OperationID: "ListInventoryItems"},
	}}
	h := handler.NewUsers(admin)

	resp, err := h.GetUserPermissions(
		handler.WithUser(t.Context(), testAdminUser),
		openapi.GetUserPermissionsRequestObject{Id: "usr-1"},
	)

	require.NoError(t, err)
	list, ok := resp.(openapi.GetUserPermissions200JSONResponse)
	require.True(t, ok)
	require.Len(t, list, 1)
	assert.Equal(t, "inventory", list[0].Service)
	assert.Equal(t, "ListInventoryItems", list[0].OperationId)
	assert.Equal(t, "usr-1", admin.permissionsTarget)
}

func TestUsersGetUserPermissionsReturns403ForANonAdmin(t *testing.T) {
	admin := &fakeAdmin{permissionsErr: usecase.ErrNotAdmin}
	h := handler.NewUsers(admin)

	resp, err := h.GetUserPermissions(t.Context(), openapi.GetUserPermissionsRequestObject{Id: "usr-1"})

	require.NoError(t, err)
	_, ok := resp.(openapi.GetUserPermissions403JSONResponse)
	assert.True(t, ok)
}

func TestUsersGetUserPermissionsReturnsAnErrorForAGenuineFailure(t *testing.T) {
	boom := errors.New("boom")
	h := handler.NewUsers(&fakeAdmin{permissionsErr: boom})

	_, err := h.GetUserPermissions(t.Context(), openapi.GetUserPermissionsRequestObject{Id: "usr-1"})

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestUsersSetUserPermissionsReplacesTheTargetsPermissions(t *testing.T) {
	admin := &fakeAdmin{}
	h := handler.NewUsers(admin)

	resp, err := h.SetUserPermissions(
		handler.WithUser(t.Context(), testAdminUser),
		openapi.SetUserPermissionsRequestObject{
			Id: "usr-1",
			Body: &openapi.SetUserPermissionsRequest{
				Permissions: []openapi.Permission{{Service: "inventory", OperationId: "ListInventoryItems"}},
			},
		},
	)

	require.NoError(t, err)
	_, ok := resp.(openapi.SetUserPermissions204Response)
	assert.True(t, ok)
	assert.Equal(t, "usr-1", admin.setPermissionsTarget)
	assert.Equal(t, []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}}, admin.setPermissionsIn)
}

func TestUsersSetUserPermissionsReturns403ForANonAdmin(t *testing.T) {
	admin := &fakeAdmin{setPermissionsErr: usecase.ErrNotAdmin}
	h := handler.NewUsers(admin)

	resp, err := h.SetUserPermissions(t.Context(), openapi.SetUserPermissionsRequestObject{
		Id:   "usr-1",
		Body: &openapi.SetUserPermissionsRequest{Permissions: []openapi.Permission{}},
	})

	require.NoError(t, err)
	_, ok := resp.(openapi.SetUserPermissions403JSONResponse)
	assert.True(t, ok)
}

func TestUsersSetUserPermissionsReturnsAnErrorForAGenuineFailure(t *testing.T) {
	boom := errors.New("boom")
	h := handler.NewUsers(&fakeAdmin{setPermissionsErr: boom})

	_, err := h.SetUserPermissions(t.Context(), openapi.SetUserPermissionsRequestObject{
		Id:   "usr-1",
		Body: &openapi.SetUserPermissionsRequest{Permissions: []openapi.Permission{}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestUsersListOperationsReturnsTheWholeCatalogue(t *testing.T) {
	admin := &fakeAdmin{operationsResult: []domain.Endpoint{
		{Service: "inventory", ServiceDisplayName: "在庫管理", OperationID: "ListInventoryItems", Summary: "List items"},
		{Service: "attendance", OperationID: "ListAttendance"},
	}}
	h := handler.NewUsers(admin)

	resp, err := h.ListOperations(handler.WithUser(t.Context(), testAdminUser), openapi.ListOperationsRequestObject{})

	require.NoError(t, err)
	list, ok := resp.(openapi.ListOperations200JSONResponse)
	require.True(t, ok)
	require.Len(t, list, 2)
	assert.Equal(t, "inventory", list[0].Service)
	assert.Equal(t, "在庫管理", list[0].ServiceDisplayName)
	assert.Equal(t, "ListInventoryItems", list[0].OperationId)
	require.NotNil(t, list[0].Summary)
	assert.Equal(t, "List items", *list[0].Summary)
	assert.Nil(t, list[1].Summary)
	assert.Equal(t, "attendance", list[1].ServiceDisplayName,
		"a service that declares no display name falls back to its identifier")
}

func TestUsersListOperationsReturns403ForANonAdmin(t *testing.T) {
	admin := &fakeAdmin{operationsErr: usecase.ErrNotAdmin}
	h := handler.NewUsers(admin)

	resp, err := h.ListOperations(t.Context(), openapi.ListOperationsRequestObject{})

	require.NoError(t, err)
	_, ok := resp.(openapi.ListOperations403JSONResponse)
	assert.True(t, ok)
}

func TestUsersListOperationsReturnsAnErrorForAGenuineFailure(t *testing.T) {
	boom := errors.New("boom")
	h := handler.NewUsers(&fakeAdmin{operationsErr: boom})

	_, err := h.ListOperations(t.Context(), openapi.ListOperationsRequestObject{})

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}
