package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// TestUserHoldsItsRole documents the shape docs/specs/auth.md section 3
// describes: a User is an id, a name and a role, and nothing else - the
// password hash stays behind the Authenticator port. There is no logic to
// exercise here - domain.User is pure data, like domain.Workspace - but
// the shape is worth pinning down as a compile-time and field-level
// contract the rest of the platform builds on.
func TestUserHoldsItsRole(t *testing.T) {
	admin := domain.User{ID: "usr-1", Name: "admin", Role: domain.RoleAdmin}

	assert.Equal(t, "usr-1", admin.ID)
	assert.Equal(t, "admin", admin.Name)
	assert.Equal(t, domain.RoleAdmin, admin.Role)
	assert.Equal(t, "admin", string(admin.Role))

	user := domain.User{ID: "usr-2", Name: "alice", Role: domain.RoleUser}
	assert.Equal(t, domain.RoleUser, user.Role)
	assert.Equal(t, "user", string(user.Role))
}

// TestPermissionNamesAnOperation documents docs/specs/auth.md section 4:
// a Permission names one operation of one service, and carries no user id
// of its own (see domain.Permission's doc comment for why).
func TestPermissionNamesAnOperation(t *testing.T) {
	perm := domain.Permission{Service: "inventory", OperationID: "ListInventoryItems"}

	assert.Equal(t, "inventory", perm.Service)
	assert.Equal(t, "ListInventoryItems", perm.OperationID)
}
