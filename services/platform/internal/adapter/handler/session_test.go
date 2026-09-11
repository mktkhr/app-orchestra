package handler_test

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

// fakeAuthenticator is a test double for the authenticator interface
// session.go declares.
type fakeAuthenticator struct {
	user       domain.User
	ok         bool
	err        error
	gotName    string
	gotPass    string
	callsCount int
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, name, password string) (domain.User, bool, error) {
	f.gotName = name
	f.gotPass = password
	f.callsCount++

	return f.user, f.ok, f.err
}

// fakeSessions is a test double for the sessionStore interface session.go
// declares.
type fakeSessions struct {
	createToken string
	createErr   error
	createdFor  string

	deleteErr    error
	deletedToken string
	deleteCalls  int
	createdCalls int
}

func (f *fakeSessions) Create(_ context.Context, userID string) (string, error) {
	f.createdFor = userID
	f.createdCalls++

	return f.createToken, f.createErr
}

func (f *fakeSessions) Delete(_ context.Context, token string) error {
	f.deletedToken = token
	f.deleteCalls++

	return f.deleteErr
}

func TestPostSessionSetsAnHTTPOnlyCookieAndReturnsTheUser(t *testing.T) {
	auth := &fakeAuthenticator{user: domain.User{ID: "usr-1", Name: "admin", Role: domain.RoleAdmin}, ok: true}
	sessions := &fakeSessions{createToken: "sess-token"}

	h := handler.NewSession(auth, sessions)

	resp, err := h.PostSession(t.Context(), openapi.PostSessionRequestObject{
		Body: &openapi.SignInRequest{Name: "admin", Password: "correct horse battery staple"},
	})
	require.NoError(t, err)

	assert.Equal(t, "admin", auth.gotName)
	assert.Equal(t, "correct horse battery staple", auth.gotPass)
	assert.Equal(t, "usr-1", sessions.createdFor)

	rec := httptest.NewRecorder()
	require.NoError(t, resp.VisitPostSessionResponse(rec))

	assert.Equal(t, 200, rec.Code)

	var body openapi.User
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "usr-1", body.Id)
	assert.Equal(t, "admin", body.Name)
	assert.Equal(t, openapi.Role(domain.RoleAdmin), body.Role)

	result := rec.Result()
	t.Cleanup(func() { _ = result.Body.Close() })

	cookies := result.Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.Equal(t, handler.SessionCookieName, cookie.Name)
	assert.Equal(t, "sess-token", cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, int(http.SameSiteLaxMode), int(cookie.SameSite))
}

func TestPostSessionReturns401AndSetsNoCookieForAWrongPassword(t *testing.T) {
	auth := &fakeAuthenticator{ok: false}
	sessions := &fakeSessions{}

	h := handler.NewSession(auth, sessions)

	resp, err := h.PostSession(t.Context(), openapi.PostSessionRequestObject{
		Body: &openapi.SignInRequest{Name: "admin", Password: "wrong"},
	})
	require.NoError(t, err)
	assert.IsType(t, openapi.PostSession401JSONResponse{}, resp)
	assert.Equal(t, 0, sessions.createdCalls)

	rec := httptest.NewRecorder()
	require.NoError(t, resp.VisitPostSessionResponse(rec))
	assert.Equal(t, 401, rec.Code)

	result := rec.Result()
	t.Cleanup(func() { _ = result.Body.Close() })
	assert.Empty(t, result.Cookies())
}

func TestPostSessionWrapsAnAuthenticatorError(t *testing.T) {
	boom := errors.New("boom")
	auth := &fakeAuthenticator{err: boom}
	h := handler.NewSession(auth, &fakeSessions{})

	_, err := h.PostSession(t.Context(), openapi.PostSessionRequestObject{
		Body: &openapi.SignInRequest{Name: "admin", Password: "x"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestPostSessionWrapsASessionStoreError(t *testing.T) {
	boom := errors.New("boom")
	auth := &fakeAuthenticator{user: domain.User{ID: "usr-1"}, ok: true}
	sessions := &fakeSessions{createErr: boom}
	h := handler.NewSession(auth, sessions)

	_, err := h.PostSession(t.Context(), openapi.PostSessionRequestObject{
		Body: &openapi.SignInRequest{Name: "admin", Password: "x"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestPostSessionFailsWithoutAnAuthenticatorOrSessionStore(t *testing.T) {
	h := handler.NewSession(nil, nil)

	_, err := h.PostSession(t.Context(), openapi.PostSessionRequestObject{
		Body: &openapi.SignInRequest{Name: "admin", Password: "x"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, handler.ErrSessionsUnavailable)
}

func TestGetSessionReturnsTheCurrentUser(t *testing.T) {
	h := handler.NewSession(nil, nil)
	ctx := handler.WithUser(t.Context(), &domain.User{ID: "usr-1", Name: "admin", Role: domain.RoleAdmin})

	resp, err := h.GetSession(ctx, openapi.GetSessionRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, openapi.GetSession200JSONResponse{Id: "usr-1", Name: "admin", Role: openapi.Role(domain.RoleAdmin)}, resp)
}

func TestGetSessionReturns401WhenNobodyIsSignedIn(t *testing.T) {
	h := handler.NewSession(nil, nil)

	resp, err := h.GetSession(t.Context(), openapi.GetSessionRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, openapi.GetSession401JSONResponse{Message: "not signed in"}, resp)
}

func TestDeleteSessionEndsTheSessionNamedByTheCookieAndClearsIt(t *testing.T) {
	sessions := &fakeSessions{}
	h := handler.NewSession(nil, sessions)
	ctx := handler.WithSessionToken(t.Context(), "sess-token")

	resp, err := h.DeleteSession(ctx, openapi.DeleteSessionRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, "sess-token", sessions.deletedToken)

	rec := httptest.NewRecorder()
	require.NoError(t, resp.VisitDeleteSessionResponse(rec))
	assert.Equal(t, 204, rec.Code)

	result := rec.Result()
	t.Cleanup(func() { _ = result.Body.Close() })

	cookies := result.Cookies()
	require.Len(t, cookies, 1)
	assert.Negative(t, cookies[0].MaxAge)
}

func TestDeleteSessionWithNoCookieCallsTheStoreNoTimes(t *testing.T) {
	sessions := &fakeSessions{}
	h := handler.NewSession(nil, sessions)

	_, err := h.DeleteSession(t.Context(), openapi.DeleteSessionRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, 0, sessions.deleteCalls)
}

func TestDeleteSessionWrapsAStoreError(t *testing.T) {
	boom := errors.New("boom")
	sessions := &fakeSessions{deleteErr: boom}
	h := handler.NewSession(nil, sessions)
	ctx := handler.WithSessionToken(t.Context(), "sess-token")

	_, err := h.DeleteSession(ctx, openapi.DeleteSessionRequestObject{})
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestDeleteSessionFailsWithoutASessionStore(t *testing.T) {
	h := handler.NewSession(nil, nil)

	_, err := h.DeleteSession(t.Context(), openapi.DeleteSessionRequestObject{})
	require.Error(t, err)
	assert.ErrorIs(t, err, handler.ErrSessionsUnavailable)
}
