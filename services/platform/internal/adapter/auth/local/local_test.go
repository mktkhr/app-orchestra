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

// TestEnsureAccountCreatesAnAccountThatThenAuthenticates proves
// EnsureAccount puts a non-admin account in place - the seam
// pkg/app.Config.SeedAccounts uses, and acceptance tests use through it,
// to build the accounts docs/plans/auth.md, Task 3's own tests need
// (docs/specs/auth.md, section 8: still no account creation over HTTP,
// only through this Go-level seam).
func TestEnsureAccountCreatesAnAccountThatThenAuthenticates(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	auth, err := local.New(ctx, store, "admin", "correct horse battery staple")
	require.NoError(t, err)

	user, err := local.EnsureAccount(ctx, store, "yamada", "yamada's password", domain.RoleUser)
	require.NoError(t, err)
	assert.Equal(t, "yamada", user.Name)
	assert.Equal(t, domain.RoleUser, user.Role)
	require.NotEmpty(t, user.ID)

	authenticated, ok, err := auth.Authenticate(ctx, "yamada", "yamada's password")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, user.ID, authenticated.ID)
	assert.Equal(t, domain.RoleUser, authenticated.Role)
}

// TestEnsureAccountIsIdempotentByName proves calling it twice for the same
// name does not create a second account, or change the first one's
// password - the same "by name" rule its own doc comment describes.
func TestEnsureAccountIsIdempotentByName(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	first, err := local.EnsureAccount(ctx, store, "yamada", "first password", domain.RoleUser)
	require.NoError(t, err)

	second, err := local.EnsureAccount(ctx, store, "yamada", "a different password", domain.RoleAdmin)
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, domain.RoleUser, second.Role, "the account already there is not re-created with the new role")

	auth, err := local.New(ctx, store, "admin-for-this-test", "irrelevant admin password")
	require.NoError(t, err)

	_, ok, err := auth.Authenticate(ctx, "yamada", "first password")
	require.NoError(t, err)
	assert.True(t, ok, "the first password still authenticates")

	_, ok, err = auth.Authenticate(ctx, "yamada", "a different password")
	require.NoError(t, err)
	assert.False(t, ok, "the second call's password was never stored")
}

// TestEnsureAccountRejectsAnEmptyPassword mirrors New's own refusal of an
// empty admin password (ErrEmptyAdminPassword): a default password is a
// way of having none while appearing to.
func TestEnsureAccountRejectsAnEmptyPassword(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	_, err := local.EnsureAccount(ctx, store, "yamada", "", domain.RoleUser)

	require.Error(t, err)
	assert.ErrorIs(t, err, local.ErrEmptyPassword)
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

// TestNewFailsWhenTheStoreCannotBeQueried exercises New's own database
// error path (SeedAdminIfNone failing) the same way
// sqlite_test.TestStoreOperationsFailOnClosedStore exercises the
// workspace store's: a closed *sqlite.Users produces the same shape of
// failure a disk error or a killed connection would, without needing to
// fake either.
func TestNewFailsWhenTheStoreCannotBeQueried(t *testing.T) {
	store := openUsers(t)
	require.NoError(t, store.Close())

	_, err := local.New(context.Background(), store, "admin", "correct horse battery staple")

	require.Error(t, err)
}

// TestEnsureAccountFailsWhenTheStoreCannotBeQueried is EnsureAccount's own
// version of TestNewFailsWhenTheStoreCannotBeQueried.
func TestEnsureAccountFailsWhenTheStoreCannotBeQueried(t *testing.T) {
	store := openUsers(t)
	require.NoError(t, store.Close())

	_, err := local.EnsureAccount(context.Background(), store, "yamada", "yamada's password", domain.RoleUser)

	require.Error(t, err)
}

// TestAuthenticateFailsWhenTheStoreCannotBeQueried is Authenticate's own
// version of the same failure mode - the account lookup itself fails,
// rather than merely finding nothing.
func TestAuthenticateFailsWhenTheStoreCannotBeQueried(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	auth, err := local.New(ctx, store, "admin", "correct horse battery staple")
	require.NoError(t, err)

	require.NoError(t, store.Close())

	_, _, err = auth.Authenticate(ctx, "admin", "correct horse battery staple")

	require.Error(t, err)
}

// TestAuthenticateFailsWhenTheStoredHashIsMalformed proves a corrupted
// stored hash - never one hashPassword itself wrote - fails loudly
// (local.ErrMalformedHash, wrapped) rather than being treated as a
// non-match, the same distinction verifyPassword's own tests make at the
// hashing layer; this exercises it through Authenticate.
func TestAuthenticateFailsWhenTheStoredHashIsMalformed(t *testing.T) {
	ctx := context.Background()
	store := openUsers(t)

	_, err := store.Create(ctx, "yamada", string(domain.RoleUser), "not-a-valid-hash")
	require.NoError(t, err)

	auth, err := local.New(ctx, store, "admin", "correct horse battery staple")
	require.NoError(t, err)

	_, _, err = auth.Authenticate(ctx, "yamada", "whatever")

	require.Error(t, err)
}
