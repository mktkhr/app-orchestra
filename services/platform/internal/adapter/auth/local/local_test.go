package local_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/auth/local"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func openUsers(t *testing.T) *sqlite.Users {
	t.Helper()

	store, err := sqlite.NewUsers(filepath.Join(t.TempDir(), "auth.db"))
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

// TestNewSeedsAnAdminWhoAuthenticatesWithTheRightPasswordAndNotTheWrongOne
// is the scenario docs/plans/auth.md, Task 0, Step 1 asks for.
func TestNewSeedsAnAdminWhoAuthenticatesWithTheRightPasswordAndNotTheWrongOne(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	auth, err := local.New(ctx, store, "admin", "correct horse battery staple")
	require.NoError(t, err)

	user, ok, err := auth.Authenticate(ctx, "admin", "correct horse battery staple")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "admin", user.Name)
	assert.Equal(t, domain.RoleAdmin, user.Role)
	assert.NotEmpty(t, user.ID)

	_, ok, err = auth.Authenticate(ctx, "admin", "wrong password")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAuthenticateFailsForAnUnknownName(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	auth, err := local.New(ctx, store, "admin", "correct horse battery staple")
	require.NoError(t, err)

	_, ok, err := auth.Authenticate(ctx, "nobody", "whatever")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestNewRejectsAnEmptyAdminPassword is docs/plans/auth.md, Task 0, Step 3:
// an unset or empty admin password is an error, not a default.
func TestNewRejectsAnEmptyAdminPassword(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	_, err := local.New(ctx, store, "admin", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, local.ErrEmptyAdminPassword)
}

// TestNewDoesNotReseedOrOverwriteAnExistingAdmin proves the seed runs at
// most once: opening a second Authenticator over the same store, with a
// different password, leaves the first admin's password the one that
// still works.
func TestNewDoesNotReseedOrOverwriteAnExistingAdmin(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	_, err := local.New(ctx, store, "admin", "first password")
	require.NoError(t, err)

	authAgain, err := local.New(ctx, store, "admin", "second password")
	require.NoError(t, err)

	_, ok, err := authAgain.Authenticate(ctx, "admin", "first password")
	require.NoError(t, err)
	assert.True(t, ok)

	_, ok, err = authAgain.Authenticate(ctx, "admin", "second password")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestStoredHashIsNotThePlainPassword proves the password never lands in
// storage as plaintext: the stored hash is an argon2id PHC string, not
// the password it was derived from.
func TestStoredHashIsNotThePlainPassword(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	const password = "correct horse battery staple"

	_, err := local.New(ctx, store, "admin", password)
	require.NoError(t, err)

	rec, found, err := store.ByName(ctx, "admin")
	require.NoError(t, err)
	require.True(t, found)

	assert.NotEqual(t, password, rec.Hash)
	assert.NotContains(t, rec.Hash, password)
	assert.True(t, strings.HasPrefix(rec.Hash, "$argon2id$"))
}
