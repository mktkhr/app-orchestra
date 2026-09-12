package acceptance_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// adminPassword is the password every acceptance test's app.New seeds its
// first admin with (docs/specs/auth.md, section 3) - one fixed password
// shared across every test file in this package, since nothing here tests
// the password itself, only what a signed-in (or not signed-in) person
// can do once app.New has seeded an account with it.
const adminPassword = "correct horse battery staple"

// newCookieJarClient gives server's own client a fresh cookie jar, so a
// session cookie a later request sets (signIn, typically) is carried by
// every request after it against the same server -
// httptest.Server.Client() otherwise builds a client with no jar at all,
// which would silently drop it.
func newCookieJarClient(t *testing.T, server *httptest.Server) {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	server.Client().Jar = jar
}

// signIn signs in as the admin account adminPassword seeds
// (docs/plans/auth.md, Task 2). Every route but GET /api/health and
// POST /api/session requires this first (docs/specs/auth.md, AC-A-102) -
// server must already have a cookie jar (newCookieJarClient) for the
// session this sets to reach any later request.
func signIn(t *testing.T, server *httptest.Server) {
	t.Helper()

	status := doJSON(t, server, http.MethodPost, "/api/session", map[string]any{
		"name":     "admin",
		"password": adminPassword,
	}, nil)
	require.Equal(t, http.StatusOK, status)
}

// newTestApp builds cfg's platform over a fresh SQLite file in
// t.TempDir() - app.New now refuses an empty DBPath (pkg/app.ErrMissingDBPath),
// the same reason every acceptance test needs one - seeds its admin with
// adminPassword, and signs in, so every request the caller makes
// afterwards against the returned server reaches its handler rather than
// requireSession's own 401 (AC-A-102). cfg.DBPath and cfg.AdminPassword
// are set here and must be left zero by the caller.
//
// cfg is a pointer, not the value app.Config{...} literals at call sites
// suggest: golangci-lint's gocritic hugeParam check (harness/quality/go/golangci.yml)
// rejects passing app.Config by value, the same reason app.New itself
// takes a pointer (see its own doc comment).
//
// Tests that only ever call GET /api/health (which is exempt from
// requireSession) do not need this - see api_test.go's own, lighter-weight
// setup.
func newTestApp(t *testing.T, cfg *app.Config) *httptest.Server {
	t.Helper()

	cfg.DBPath = filepath.Join(t.TempDir(), "acceptance.db")
	cfg.AdminPassword = adminPassword

	h, err := app.New(cfg)
	require.NoError(t, err)

	server := httptest.NewServer(h)
	t.Cleanup(server.Close)

	newCookieJarClient(t, server)
	signIn(t, server)

	return server
}
