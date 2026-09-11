package sqlite

import (
	"crypto/rand"
	"strings"
)

// newID returns a fresh, opaque identifier prefixed with kind (e.g. "ws" or
// "pnl"). It is randomly generated rather than taken from SQLite's own
// rowid so that a workspace's id and a panel's id are never accidentally
// interchangeable, and so a caller across the HTTP boundary (Task 1) never
// has to know whether an id names a workspace or a panel from its shape
// alone versus which endpoint returned it.
//
// crypto/rand.Text (Go 1.24+) has no error return - unlike reading
// crypto/rand.Reader directly, which can fail - so callers never have to
// handle an id-generation error that is, in practice, unreachable in
// tests. Nothing in this codebase inspects an id's structure or depends
// on it being unpredictable in a security sense; it exists only to be
// unique and stable once assigned. Tests never assert a specific id -
// they capture what Create or AddPanel returns and check that later reads
// echo it back - so this random source does not make the store's
// behaviour non-deterministic to test.
func newID(kind string) string {
	return kind + "-" + strings.ToLower(rand.Text())
}
