package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// admin is what Users needs from the usecase layer: satisfied by
// *usecase.Admin. An interface here, rather than the concrete type, for
// the same reason workspaces in workspace.go is one.
type admin interface {
	ListUsers(ctx context.Context, user *domain.User) ([]domain.User, error)
	Permissions(ctx context.Context, user *domain.User, targetUserID string) ([]domain.Permission, error)
	SetPermissions(ctx context.Context, user *domain.User, targetUserID string, permissions []domain.Permission) error
}

// Users implements the "users" tag of the generated strict server
// interface: the admin's three endpoints (docs/specs/auth.md, section 6).
// Every method is admin only - refused with 403 to anybody else - and the
// check itself lives in usecase.Admin, not here (see that type's own doc
// comment for why).
type Users struct {
	admin admin
}

// NewUsers builds the /api/users handler over a.
func NewUsers(a admin) *Users {
	return &Users{admin: a}
}

// ListUsers implements GET /api/users.
func (h *Users) ListUsers(
	ctx context.Context,
	_ openapi.ListUsersRequestObject,
) (openapi.ListUsersResponseObject, error) {
	users, err := h.admin.ListUsers(ctx, currentUser(ctx))
	if err != nil {
		forbidden := func(msg string) openapi.ListUsersResponseObject {
			return openapi.ListUsers403JSONResponse{Message: msg}
		}

		if resp, ok := adminErrorResponse(err, forbidden); ok {
			return resp, nil
		}

		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	out := make(openapi.ListUsers200JSONResponse, len(users))
	for i := range users {
		out[i] = toAPIUser(&users[i])
	}

	return out, nil
}

// GetUserPermissions implements GET /api/users/{id}/permissions.
func (h *Users) GetUserPermissions(
	ctx context.Context,
	request openapi.GetUserPermissionsRequestObject,
) (openapi.GetUserPermissionsResponseObject, error) {
	permissions, err := h.admin.Permissions(ctx, currentUser(ctx), request.Id)
	if err != nil {
		forbidden := func(msg string) openapi.GetUserPermissionsResponseObject {
			return openapi.GetUserPermissions403JSONResponse{Message: msg}
		}

		if resp, ok := adminErrorResponse(err, forbidden); ok {
			return resp, nil
		}

		return nil, fmt.Errorf("reading permissions of %s: %w", request.Id, err)
	}

	out := make(openapi.GetUserPermissions200JSONResponse, len(permissions))
	for i := range permissions {
		out[i] = toAPIPermission(&permissions[i])
	}

	return out, nil
}

// SetUserPermissions implements PUT /api/users/{id}/permissions: replaces
// the named account's permissions wholesale with request.Body.Permissions.
func (h *Users) SetUserPermissions(
	ctx context.Context,
	request openapi.SetUserPermissionsRequestObject,
) (openapi.SetUserPermissionsResponseObject, error) {
	permissions := toDomainPermissions(request.Body)

	if err := h.admin.SetPermissions(ctx, currentUser(ctx), request.Id, permissions); err != nil {
		forbidden := func(msg string) openapi.SetUserPermissionsResponseObject {
			return openapi.SetUserPermissions403JSONResponse{Message: msg}
		}

		if resp, ok := adminErrorResponse(err, forbidden); ok {
			return resp, nil
		}

		return nil, fmt.Errorf("setting permissions of %s: %w", request.Id, err)
	}

	return openapi.SetUserPermissions204Response{}, nil
}

// adminErrorResponse maps usecase.ErrNotAdmin onto its 403 response - the
// one error every Users method's caller is at fault for, the same
// "map a sentinel to its status, everything else is a real error" shape
// addPanelErrorResponse follows in workspace.go. build is the response's
// constructor for the specific operation calling this, since each
// operation's 403 is its own generated type
// (openapi.ListUsers403JSONResponse and friends) with no shared interface
// narrower than the response object itself.
func adminErrorResponse[T any](err error, build func(msg string) T) (T, bool) {
	if errors.Is(err, usecase.ErrNotAdmin) {
		return build(err.Error()), true
	}

	var zero T

	return zero, false
}

// toAPIPermission converts one domain.Permission into the wire Permission.
func toAPIPermission(p *domain.Permission) openapi.Permission {
	return openapi.Permission{Service: p.Service, OperationId: p.OperationID}
}

// toDomainPermissions converts a SetUserPermissionsRequest's permissions
// into their domain shape.
func toDomainPermissions(body *openapi.SetUserPermissionsRequest) []domain.Permission {
	permissions := make([]domain.Permission, len(body.Permissions))
	for i, p := range body.Permissions {
		permissions[i] = domain.Permission{Service: p.Service, OperationID: p.OperationId}
	}

	return permissions
}
