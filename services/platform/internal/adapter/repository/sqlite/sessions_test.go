package sqlite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func openSessions(t *testing.T, path string) *sqlite.Sessions {
	t.Helper()

	store, err := sqlite.NewSessions(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

// TestSessionRoundTripsThenNotAfterDeletion is the scenario
// docs/plans/auth.md, Task 0, Step 1 asks for.
func TestSessionRoundTripsThenNotAfterDeletion(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)

	users := openUsers(t, path)
	_, err := users.SeedAdminIfNone(ctx, "admin", "hashed-value")
	require.NoError(t, err)

	rec, found, err := users.ByName(ctx, "admin")
	require.NoError(t, err)
	require.True(t, found)

	sessions := openSessions(t, path)

	token, err := sessions.Create(ctx, rec.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	got, found, err := sessions.User(ctx, token)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.User{ID: rec.ID, Name: "admin", Role: domain.RoleAdmin}, got)

	require.NoError(t, sessions.Delete(ctx, token))

	_, found, err = sessions.User(ctx, token)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSessionUserReturnsFalseForAnUnknownToken(t *testing.T) {
	ctx := context.Background()
	store := openSessions(t, dbPath(t))

	_, found, err := store.User(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSessionDeleteUnknownTokenIsNotAnError(t *testing.T) {
	ctx := context.Background()
	store := openSessions(t, dbPath(t))

	require.NoError(t, store.Delete(ctx, "does-not-exist"))
}
