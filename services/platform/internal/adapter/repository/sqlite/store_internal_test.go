package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A white-box (package sqlite) test file: db is unexported, and this is
// the one place AC-S-102 - "the process opens one *sql.DB for its database
// file, however many stores read it" - can be pinned directly, by
// comparing the *sql.DB pointer each constructor holds rather than only
// observing effects from outside.

// TestOpenAppliesThePragmasEveryStoreNeeds pins S2/S3 (docs/specs/storage.md)
// at the one place they are actually set: Open's DSN. WAL and a five-second
// busy_timeout only help if they are really in effect on the connection
// every store ends up sharing - this reads them back with PRAGMA rather
// than trusting the DSN string was well-formed.
func TestOpenAppliesThePragmasEveryStoreNeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pragmas.db")

	db, err := Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	var journalMode string
	require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode))
	assert.Equal(t, "wal", journalMode)

	var busyTimeout int
	require.NoError(t, db.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout))
	assert.Equal(t, 5000, busyTimeout)
}

// TestEveryStoreFromDBSharesTheGivenSQLDB is AC-S-102 itself: Open returns
// one *sql.DB, and NewFromDB/NewUsersFromDB/NewSessionsFromDB/
// NewPermissionsFromDB each wrap it directly rather than opening a
// connection of their own - the shape pkg/app.build now uses to put every
// store on one file (docs/specs/storage.md, section 2, "the process opens
// the file four times").
func TestEveryStoreFromDBSharesTheGivenSQLDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")

	db, err := Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	store := NewFromDB(db)
	users := NewUsersFromDB(db)
	sessions := NewSessionsFromDB(db)
	permissions := NewPermissionsFromDB(db)

	assert.Same(t, db, store.db)
	assert.Same(t, db, users.db)
	assert.Same(t, db, sessions.db)
	assert.Same(t, db, permissions.db)
}
