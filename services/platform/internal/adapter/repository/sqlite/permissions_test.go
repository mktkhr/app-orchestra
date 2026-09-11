package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func openPermissions(t *testing.T, path string) *sqlite.Permissions {
	t.Helper()

	store, err := sqlite.NewPermissions(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestPermissionsForReturnsEmptyWhenNoneAreSet(t *testing.T) {
	ctx := context.Background()
	store := openPermissions(t, dbPath(t))

	permissions, err := store.For(ctx, "usr-1")
	require.NoError(t, err)
	assert.Empty(t, permissions)
}

// TestPermissionsSetReplacesWholesale is the scenario docs/plans/auth.md,
// Task 0, Step 1 asks for: setting a new set of permissions replaces
// whatever was there before, not merges with it.
func TestPermissionsSetReplacesWholesale(t *testing.T) {
	ctx := context.Background()
	store := openPermissions(t, dbPath(t))

	require.NoError(t, store.Set(ctx, "usr-1", []domain.Permission{
		{Service: "inventory", OperationID: "ListInventoryItems"},
		{Service: "attendance", OperationID: "ListAttendanceRecords"},
	}))

	got, err := store.For(ctx, "usr-1")
	require.NoError(t, err)
	assert.Equal(t, []domain.Permission{
		{Service: "attendance", OperationID: "ListAttendanceRecords"},
		{Service: "inventory", OperationID: "ListInventoryItems"},
	}, got)

	require.NoError(t, store.Set(ctx, "usr-1", []domain.Permission{
		{Service: "inventory", OperationID: "CreateInventoryItem"},
	}))

	got, err = store.For(ctx, "usr-1")
	require.NoError(t, err)
	assert.Equal(t, []domain.Permission{{Service: "inventory", OperationID: "CreateInventoryItem"}}, got)
}

func TestPermissionsSetToEmptyClearsThem(t *testing.T) {
	ctx := context.Background()
	store := openPermissions(t, dbPath(t))

	require.NoError(t, store.Set(ctx, "usr-1", []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}}))
	require.NoError(t, store.Set(ctx, "usr-1", nil))

	got, err := store.For(ctx, "usr-1")
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestPermissionsSetScopesByUser is half of the point of user_id being
// part of every row: setting one person's permissions must not touch
// another's.
func TestPermissionsSetScopesByUser(t *testing.T) {
	ctx := context.Background()
	store := openPermissions(t, dbPath(t))

	require.NoError(t, store.Set(ctx, "usr-1", []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}}))
	require.NoError(t, store.Set(ctx, "usr-2", []domain.Permission{{Service: "attendance", OperationID: "ListAttendanceRecords"}}))

	got, err := store.For(ctx, "usr-1")
	require.NoError(t, err)
	assert.Equal(t, []domain.Permission{{Service: "inventory", OperationID: "ListInventoryItems"}}, got)
}
