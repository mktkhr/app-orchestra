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
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil)), "", fakeSessionUsers{})
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
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil)), "", fakeSessionUsers{})
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

	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewSession(nil, nil, true), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil), handler.NewUsers(nil)), dir, fakeSessionUsers{})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}
