package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// userOnWire is the wire shape of User.
type userOnWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// newSessionTestApp builds a platform over a fresh SQLite file in
// t.TempDir(), with the admin seeded from sessionTestAdminPassword and no
// service configured - these tests are about the session endpoints
// themselves, not about what a signed-in person can then do with them
// (workspace_test.go covers that).
func newSessionTestApp(t *testing.T) *httptest.Server {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "session.db")

	h, err := app.New(&app.Config{DBPath: dbPath, AdminPassword: sessionTestAdminPassword})
	require.NoError(t, err)

	server := httptest.NewServer(h)
	t.Cleanup(server.Close)

	server.Client().Jar, err = cookiejar.New(nil)
	require.NoError(t, err)

	return server
}

const sessionTestAdminPassword = "correct horse battery staple"

// TestSigningInSetsACookieAndReturnsTheUser is docs/plans/auth.md, Task 2,
// Step 2's first scenario: signing in sets a cookie - HttpOnly, so a
// script can never read it (A2, docs/specs/auth.md) - and returns the
// user.
func TestSigningInSetsACookieAndReturnsTheUser(t *testing.T) {
	server := newSessionTestApp(t)

	var body userOnWire
	status := doJSON(t, server, http.MethodPost, "/api/session", map[string]any{
		"name":     "admin",
		"password": sessionTestAdminPassword,
	}, &body)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "admin", body.Name)
	assert.Equal(t, "admin", body.Role)
	require.NotEmpty(t, body.ID)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	cookies := server.Client().Jar.Cookies(serverURL)
	require.Len(t, cookies, 1)
	assert.Equal(t, "orchestra_session", cookies[0].Name)
	assert.NotEmpty(t, cookies[0].Value)

	// http.CookieJar does not expose HttpOnly (net/http/cookiejar never
	// keeps it - it is meaningless to anything but a browser), so this
	// checks the raw Set-Cookie header from a fresh, jarless request
	// instead.
	raw, err := json.Marshal(map[string]any{"name": "admin", "password": sessionTestAdminPassword})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/session", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	setCookies := resp.Cookies()
	require.Len(t, setCookies, 1)
	assert.True(t, setCookies[0].HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, setCookies[0].SameSite)
}

// TestSigningInWithAWrongPasswordReturns401AndNoSession is Task 2, Step
// 2's second scenario.
func TestSigningInWithAWrongPasswordReturns401AndNoSession(t *testing.T) {
	server := newSessionTestApp(t)

	status := doJSON(t, server, http.MethodPost, "/api/session", map[string]any{
		"name":     "admin",
		"password": "not the password",
	}, nil)
	require.Equal(t, http.StatusUnauthorized, status)

	// No session was started: the same client, which would now be
	// carrying a cookie had one been set, still gets 401 from GET
	// /api/session.
	status = doJSON(t, server, http.MethodGet, "/api/session", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, status)
}

// TestEveryEndpointButHealthIs401WithoutASession is Task 2, Step 2's
// third scenario (AC-A-102).
func TestEveryEndpointButHealthIs401WithoutASession(t *testing.T) {
	server := newSessionTestApp(t)

	status := doJSON(t, server, http.MethodGet, "/api/health", nil, nil)
	assert.Equal(t, http.StatusOK, status)

	for _, path := range []string{"/api/session", "/api/workspaces"} {
		status := doJSON(t, server, http.MethodGet, path, nil, nil)
		assert.Equal(t, http.StatusUnauthorized, status, "GET %s without a session", path)
	}
}

// TestSigningOutEndsTheSessionAndFurtherRequestsAre401 is Task 2, Step 2's
// fourth scenario, and AC-A-107's second half.
func TestSigningOutEndsTheSessionAndFurtherRequestsAre401(t *testing.T) {
	server := newSessionTestApp(t)

	status := doJSON(t, server, http.MethodPost, "/api/session", map[string]any{
		"name":     "admin",
		"password": sessionTestAdminPassword,
	}, nil)
	require.Equal(t, http.StatusOK, status)

	status = doJSON(t, server, http.MethodGet, "/api/workspaces", nil, nil)
	require.Equal(t, http.StatusOK, status, "signed in, a protected route must answer")

	status = doJSON(t, server, http.MethodDelete, "/api/session", nil, nil)
	require.Equal(t, http.StatusNoContent, status)

	status = doJSON(t, server, http.MethodGet, "/api/workspaces", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, status, "signed out, the same route must refuse")

	status = doJSON(t, server, http.MethodGet, "/api/session", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, status)
}

// TestSessionSurvivesAReload is AC-A-107's first half: a fresh request
// with the same cookie (a "reload") still answers as the signed-in user.
func TestSessionSurvivesAReload(t *testing.T) {
	server := newSessionTestApp(t)

	status := doJSON(t, server, http.MethodPost, "/api/session", map[string]any{
		"name":     "admin",
		"password": sessionTestAdminPassword,
	}, nil)
	require.Equal(t, http.StatusOK, status)

	var body userOnWire

	status = doJSON(t, server, http.MethodGet, "/api/session", nil, &body)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "admin", body.Name)
}

// TestSigningOutWithNoSessionIsNotAnError proves DELETE /api/session is
// exempt from requireSession's own 401 - signing out is idempotent, not a
// protected action (adapter/handler/session.go's DeleteSession doc
// comment).
func TestSigningOutWithNoSessionIsNotAnError(t *testing.T) {
	server := newSessionTestApp(t)

	status := doJSON(t, server, http.MethodDelete, "/api/session", nil, nil)
	assert.Equal(t, http.StatusNoContent, status)
}
