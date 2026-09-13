package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
)

// fakeSessionUsers resolves one fixed token to a fixed admin user, and
// every other token to "not found" - just enough for these tests, which
// are about NewRouter's own routing (health, unknown routes, static
// files), not about session resolution itself (session_internal_test.go's
// own fakeSessionUsers covers that). httpserver.NewRouter's sessions
// parameter must never be nil now - see requireSession's doc comment.
type fakeSessionUsers struct{}

const fakeSessionToken = "valid-token"

func (fakeSessionUsers) User(_ context.Context, token string) (domain.User, bool, error) {
	if token != fakeSessionToken {
		return domain.User{}, false, nil
	}

	return domain.User{ID: "usr-1", Role: domain.RoleAdmin}, true, nil
}

// requestCookie attaches fakeSessionToken to req the same way a signed-in
// browser would.
func requestCookie() *http.Cookie {
	return &http.Cookie{
		Name:     handler.SessionCookieName,
		Value:    fakeSessionToken,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func TestNewRouterServesHealth(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil), handler.NewCatalog(nil)), "", fakeSessionUsers{})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

// TestNewRouterRejectsUnknownRoute signs in (an unknown /api/ route is
// still gated by requireSession like any other, docs/specs/auth.md
// AC-A-102) and checks that a route the embedded spec does not declare
// answers 404, not the openapi validator's own error shape.
func TestNewRouterRejectsUnknownRoute(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil), handler.NewCatalog(nil)), "", fakeSessionUsers{})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/does-not-exist", http.NoBody)
	req.AddCookie(requestCookie())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewRouterServesStaticDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("hello"), 0o600))

	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil), handler.NewCatalog(nil)), dir, fakeSessionUsers{})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}

// newStaticRouter builds a router over a temporary static directory holding
// index.html and one real asset, assets/app-abc123.js - just enough to pin
// docs/specs/routing.md section 4's rule: a path with a file extension is a
// request for a file and answers 404 when missing; everything else is the
// application.
func newStaticRouter(t *testing.T) http.Handler {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>index</html>"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o600))

	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil), handler.NewCatalog(nil)), dir, fakeSessionUsers{})
	require.NoError(t, err)

	return router
}

// TestNewRouterServesIndexForApplicationRoute pins AC-R-101/AC-R-102: a
// path with no file extension - the address of a screen, not a file - gets
// index.html so a reload of a deep link like /workspaces/abc still works.
func TestNewRouterServesIndexForApplicationRoute(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/workspaces/abc", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<html>index</html>", rec.Body.String())
}

// TestNewRouterServesIndexAtRoot checks "/" itself still resolves to the
// index once the fallback exists alongside it.
func TestNewRouterServesIndexAtRoot(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<html>index</html>", rec.Body.String())
}

// TestNewRouterServesExistingAsset checks the heuristic's other edge does
// not regress: a real asset is still served as itself.
func TestNewRouterServesExistingAsset(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/app-abc123.js", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "console.log(1)", rec.Body.String())
}

// TestNewRouterMissingAssetIs404NotHTML pins AC-R-102 directly: a browser
// asking for a build asset that is not there must get a 404, not
// index.html - handing it HTML produces a syntax error in a file that
// appears to exist.
func TestNewRouterMissingAssetIs404NotHTML(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/missing-abc123.js", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "<html>")
}

// TestNewRouterUnknownAPIRouteStillAnswersAsAPI pins AC-R-103: the
// fallback must not shadow an /api/ path that does not exist in the
// embedded spec - it still answers as the API (404 from the API's own
// mux, not the application's index.html).
func TestNewRouterUnknownAPIRouteStillAnswersAsAPI(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/nope", http.NoBody)
	req.AddCookie(requestCookie())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "<html>")
}

// TestNewRouterTraversalDoesNotEscapeStaticDir checks that a path trying
// to climb out of staticDir - plain and percent-encoded - never reaches a
// file outside it. net/http's ServeMux and http.FileServer both clean the
// request path before touching the filesystem, so an ordinary "../" is
// collapsed away by the mux itself (net/http.cleanPath) long before this
// package's own code runs; this test exists to pin that the fallback does
// not reopen the hole by, say, joining the raw URL path onto staticDir
// itself.
func TestNewRouterTraversalDoesNotEscapeStaticDir(t *testing.T) {
	router := newStaticRouter(t)

	for _, path := range []string{
		"/../etc/passwd",
		"/assets/../../etc/passwd",
		"/%2e%2e/%2e%2e/etc/passwd",
		"/assets/%2e%2e%2fetc%2fpasswd",
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.NotEqual(t, http.StatusOK, rec.Code, "path %q must not resolve to a file", path)
		assert.NotContains(t, rec.Body.String(), "root:", "path %q must not leak /etc/passwd", path)
	}
}

// TestNewRouterEmptyStaticDirBehavesAsToday pins the one thing that must
// never move: with no frontend built, an application-shaped route
// (formerly a 404 from an absent "/" handler) still answers 404, not the
// fallback's index.html - several tests and every acceptance suite start
// a platform with no frontend at all.
func TestNewRouterEmptyStaticDirBehavesAsToday(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil), handler.NewCatalog(nil)), "", fakeSessionUsers{})
	require.NoError(t, err)

	for _, path := range []string{"/", "/workspaces/abc", "/assets/app.js"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code, "path %q", path)
	}
}
