package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// A white-box (package httpserver) test file: requireSession is
// unexported, and its own tests are the ones that need to reach it
// directly rather than through a whole router (internal/infra/httpserver's
// own router_test.go covers that end-to-end).

// fakeSessionUsers is a test double for the sessionUsers interface this
// file declares.
type fakeSessionUsers struct {
	byToken  map[string]domain.User
	err      error
	gotToken string
	calls    int
}

func (f *fakeSessionUsers) User(_ context.Context, token string) (domain.User, bool, error) {
	f.gotToken = token
	f.calls++

	if f.err != nil {
		return domain.User{}, false, f.err
	}

	user, ok := f.byToken[token]

	return user, ok, nil
}

// okNext is a next handler that always answers 200 "ok" - requireSession's
// tests care only about whether it was reached and with which cookie,
// never about what a real openapi handler would do with the request.
var okNext = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("ok")); err != nil {
		// httptest.ResponseRecorder, the only writer these tests use,
		// never errors; nothing meaningful to do with a real one either
		// once the status is already written.
		return
	}
})

func TestRequireSessionAnswers401ForAProtectedPathWithNoCookie(t *testing.T) {
	mw := requireSession(&fakeSessionUsers{}, okNext)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body openapi.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.NotEmpty(t, body.Message)
}

func TestRequireSessionAnswers401ForAnUnknownCookie(t *testing.T) {
	store := &fakeSessionUsers{byToken: map[string]domain.User{}}
	mw := requireSession(store, okNext)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	req.AddCookie(requestCookie("not-a-real-token"))
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "not-a-real-token", store.gotToken)
}

func TestRequireSessionLetsAValidCookieReachNext(t *testing.T) {
	store := &fakeSessionUsers{byToken: map[string]domain.User{
		"good-token": {ID: "usr-1", Name: "admin", Role: domain.RoleAdmin},
	}}
	mw := requireSession(store, okNext)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	req.AddCookie(requestCookie("good-token"))
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRequireSessionExemptsHealthAndSessionPathsWithNoCookie(t *testing.T) {
	mw := requireSession(&fakeSessionUsers{}, okNext)

	for _, path := range []string{"/api/health", "/api/session"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "path %s must not be gated by requireSession", path)
	}
}

func TestRequireSessionReturns500WhenTheStoreFails(t *testing.T) {
	mw := requireSession(&fakeSessionUsers{err: errors.New("boom")}, okNext)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	req.AddCookie(requestCookie("any-token"))
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// requestCookie builds the session cookie one of these tests attaches to
// an outgoing *http.Request with req.AddCookie. Secure, HttpOnly and
// SameSite are meaningless on a request cookie - a browser never sends
// them back, only Name and Value - but gosec's insecure-cookie check
// (G124) flags any *http.Cookie literal missing them regardless of which
// direction it travels, so this sets them anyway rather than repeating
// the fields at every call site.
func requestCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     handler.SessionCookieName,
		Value:    value,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
