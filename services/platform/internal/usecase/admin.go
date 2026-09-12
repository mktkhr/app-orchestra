package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// UserStore lists every account the platform knows (docs/specs/auth.md,
// section 6: GET /api/users). Implemented by
// internal/adapter/repository/sqlite, alongside PermissionStore and
// SessionStore, in the same SQLite file - kept as a port here for the same
// reason those are: this layer depends on the shape of the storage, not on
// SQLite or database/sql (harness/quality/go/golangci.yml, depguard).
type UserStore interface {
	// List returns every account, in no particular order the caller may
	// rely on beyond what the store itself documents.
	List(ctx context.Context) ([]domain.User, error)
}

// ErrNotAdmin is returned by every Admin method when the calling user is
// nil or not domain.RoleAdmin (docs/specs/auth.md, A5). Named here, not
// borrowed by internal/adapter/handler from some HTTP-shaped sentinel,
// because the check belongs to this layer: A6 asks for the user to reach
// the usecase as an argument precisely so a check on it cannot be left out
// by a handler that forgot to make it - and a check that lives only in the
// handler is exactly the kind that gets forgotten on the next endpoint
// added there.
var ErrNotAdmin = errors.New("admin only")

// Admin is the usecase behind docs/specs/auth.md section 6's three
// admin-only endpoints: reading every account and reading or replacing one
// of their permission sets. Every method refuses anybody but an admin with
// ErrNotAdmin, checked once, here, rather than trusted to whichever
// handler happens to call in - the same reasoning that keeps
// Orchestrator.Invoke's permission check in the usecase layer instead of
// at the HTTP edge (docs/specs/auth.md, section 5).
type Admin struct {
	users       UserStore
	permissions PermissionStore
}

// NewAdmin builds an Admin usecase over users and permissions.
func NewAdmin(users UserStore, permissions PermissionStore) *Admin {
	return &Admin{users: users, permissions: permissions}
}

// ListUsers returns every account. admin only.
func (a *Admin) ListUsers(ctx context.Context, user *domain.User) ([]domain.User, error) {
	if !isAdmin(user) {
		return nil, ErrNotAdmin
	}

	users, err := a.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	return users, nil
}

// Permissions returns every permission targetUserID holds. admin only.
func (a *Admin) Permissions(ctx context.Context, user *domain.User, targetUserID string) ([]domain.Permission, error) {
	if !isAdmin(user) {
		return nil, ErrNotAdmin
	}

	permissions, err := a.permissions.For(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("reading permissions of %s: %w", targetUserID, err)
	}

	return permissions, nil
}

// SetPermissions replaces every permission targetUserID holds with
// permissions, wholesale - the same replace-wholesale shape
// PermissionStore.Set itself documents. admin only.
func (a *Admin) SetPermissions(
	ctx context.Context,
	user *domain.User,
	targetUserID string,
	permissions []domain.Permission,
) error {
	if !isAdmin(user) {
		return ErrNotAdmin
	}

	if err := a.permissions.Set(ctx, targetUserID, permissions); err != nil {
		return fmt.Errorf("setting permissions of %s: %w", targetUserID, err)
	}

	return nil
}

// isAdmin reports whether user is signed in and holds domain.RoleAdmin -
// an admin holds every permission implicitly and is the only role that may
// reach an Admin method (docs/specs/auth.md, A5).
func isAdmin(user *domain.User) bool {
	return user != nil && user.Role == domain.RoleAdmin
}
