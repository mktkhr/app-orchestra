package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func openUsers(t *testing.T, path string) *sqlite.Users {
	t.Helper()

	store, err := sqlite.NewUsers(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestUsersSeedAdminIfNoneSeedsOnceAndIsFoundByName(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t, dbPath(t))

	seeded, err := store.SeedAdminIfNone(ctx, "admin", "hashed-value")
	require.NoError(t, err)
	assert.True(t, seeded)

	rec, found, err := store.ByName(ctx, "admin")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotEmpty(t, rec.ID)
	assert.Equal(t, "admin", rec.Name)
	assert.Equal(t, domain.RoleAdmin, rec.Role)
	assert.Equal(t, "hashed-value", rec.Hash)
}

// TestUsersSeedAdminIfNoneDoesNotOverwriteAnExistingAccount is the point
// of the "IfNone" in the name (docs/specs/auth.md, section 3): a database
// that already has an account keeps it, even if seeded a second time with
// a different hash.
func TestUsersSeedAdminIfNoneDoesNotOverwriteAnExistingAccount(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t, dbPath(t))

	seeded, err := store.SeedAdminIfNone(ctx, "admin", "first-hash")
	require.NoError(t, err)
	assert.True(t, seeded)

	seededAgain, err := store.SeedAdminIfNone(ctx, "someone-else", "second-hash")
	require.NoError(t, err)
	assert.False(t, seededAgain)

	rec, found, err := store.ByName(ctx, "admin")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "first-hash", rec.Hash)

	_, found, err = store.ByName(ctx, "someone-else")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestUsersByNameReturnsFalseWhenMissing(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t, dbPath(t))

	_, found, err := store.ByName(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

// TestUsersCreateAndList proves Create inserts an account of any role -
// not only the admin SeedAdminIfNone seeds - and that List returns every
// account, in name order, without a password hash.
func TestUsersCreateAndList(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t, dbPath(t))

	_, err := store.SeedAdminIfNone(ctx, "admin", "admin-hash")
	require.NoError(t, err)

	id, err := store.Create(ctx, "yamada", string(domain.RoleUser), "yamada-hash")
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	users, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "admin", users[0].Name)
	assert.Equal(t, domain.RoleAdmin, users[0].Role)
	assert.Equal(t, id, users[1].ID)
	assert.Equal(t, "yamada", users[1].Name)
	assert.Equal(t, domain.RoleUser, users[1].Role)
}

func TestUsersListReturnsEmptyWhenNoAccountsExist(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t, dbPath(t))

	users, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)
}

// TestUsersCreateRejectsADuplicateName proves the users table's own
// UNIQUE constraint (schema.sql) surfaces as a plain error from Create,
// rather than silently overwriting the existing account.
func TestUsersCreateRejectsADuplicateName(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t, dbPath(t))

	_, err := store.Create(ctx, "yamada", string(domain.RoleUser), "first-hash")
	require.NoError(t, err)

	_, err = store.Create(ctx, "yamada", string(domain.RoleUser), "second-hash")
	require.Error(t, err)
}
