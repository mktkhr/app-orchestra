package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// SessionCookieName is the cookie a signed-in browser carries its opaque
// session token in (docs/specs/auth.md, A2). Exported so
// internal/infra/httpserver's session middleware - which must read the
// same cookie before a request ever reaches this handler - names the same
// literal without this package and that one importing each other's
// constant the wrong way round (infra depends on adapter, not the
// reverse: AGENTS.md #2's layer order).
const SessionCookieName = "orchestra_session"

// sessionCookieMaxAge mirrors internal/adapter/repository/sqlite's own
// sessionTTL: the cookie should not outlive the session it names, and
// there is nothing to gain from asking the browser to keep it longer than
// the server will honour it.
const sessionCookieMaxAge = 24 * time.Hour

// ErrSessionsUnavailable is returned by PostSession and DeleteSession when
// this handler was built with no authenticator or session store - the
// shape pkg/app's own pre-auth tests build (a Config with no DBPath, the
// same case internal/infra/httpserver.NewRouter treats as "run every
// request as a fixed admin" - see that package's requireSession). Nothing
// that leaves DBPath empty ever calls /api/session either, so this exists
// only so a bad wiring fails loudly instead of a nil pointer panic.
var ErrSessionsUnavailable = errors.New("sessions are not available: no database configured")

// authenticator is what Session needs from usecase.Authenticator.
type authenticator interface {
	Authenticate(ctx context.Context, name, password string) (domain.User, bool, error)
}

// sessionStore is what Session needs from usecase.SessionStore: Create and
// Delete. Session never needs User - that lookup happens once, in
// internal/infra/httpserver's middleware, before this handler ever runs.
type sessionStore interface {
	Create(ctx context.Context, userID string) (string, error)
	Delete(ctx context.Context, token string) error
}

// Session implements the "session" tag of the generated strict server
// interface: POST, GET and DELETE /api/session (docs/specs/auth.md,
// section 6).
type Session struct {
	authenticator authenticator
	sessions      sessionStore
}

// NewSession builds the /api/session handler over auth and sessions.
// Either may be nil - see ErrSessionsUnavailable - for the same reason
// NewWorkspace(nil) is a valid handler over no store.
func NewSession(auth authenticator, sessions sessionStore) *Session {
	return &Session{authenticator: auth, sessions: sessions}
}

// PostSession implements POST /api/session: checks the name and password
// against the platform's accounts and, on success, starts a session and
// sets its cookie. A wrong name or password is 401 with no cookie set -
// the same answer either way, so a caller can never tell "no such
// account" from "wrong password" (usecase.Authenticator's own doc
// comment).
func (h *Session) PostSession(
	ctx context.Context,
	request openapi.PostSessionRequestObject,
) (openapi.PostSessionResponseObject, error) {
	if h.authenticator == nil || h.sessions == nil {
		return nil, ErrSessionsUnavailable
	}

	user, ok, err := h.authenticator.Authenticate(ctx, request.Body.Name, request.Body.Password)
	if err != nil {
		return nil, fmt.Errorf("authenticating: %w", err)
	}

	if !ok {
		return openapi.PostSession401JSONResponse{Message: "wrong name or password"}, nil
	}

	token, err := h.sessions.Create(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("starting a session: %w", err)
	}

	return signInResponse{user: toAPIUser(&user), token: token}, nil
}

// GetSession implements GET /api/session: the signed-in user, from
// currentUser - internal/infra/httpserver's middleware already resolved
// the cookie before this handler ran - or 401 when nobody is signed in.
func (h *Session) GetSession(
	ctx context.Context,
	_ openapi.GetSessionRequestObject,
) (openapi.GetSessionResponseObject, error) {
	user := currentUser(ctx)
	if user == nil {
		return openapi.GetSession401JSONResponse{Message: "not signed in"}, nil
	}

	return openapi.GetSession200JSONResponse(toAPIUser(user)), nil
}

// DeleteSession implements DELETE /api/session: ends the session named by
// the request's cookie, if any, and clears it. Signing out when nobody is
// signed in - no cookie, or one naming a session already gone - is not an
// error: the end state is the same either way
// (usecase.SessionStore.Delete's own doc comment).
func (h *Session) DeleteSession(
	ctx context.Context,
	_ openapi.DeleteSessionRequestObject,
) (openapi.DeleteSessionResponseObject, error) {
	if h.sessions == nil {
		return nil, ErrSessionsUnavailable
	}

	if token := sessionToken(ctx); token != "" {
		if err := h.sessions.Delete(ctx, token); err != nil {
			return nil, fmt.Errorf("ending session: %w", err)
		}
	}

	return signOutResponse{}, nil
}

// toAPIUser converts one domain.User into the wire User.
func toAPIUser(user *domain.User) openapi.User {
	return openapi.User{Id: user.ID, Name: user.Name, Role: openapi.Role(user.Role)}
}

// signInResponse is PostSession's 200: it sets the session cookie -
// HttpOnly and SameSite, so a script can never read or leak the token it
// carries (docs/specs/auth.md, A2) - alongside the signed-in user's JSON
// body. Not the generated openapi.PostSession200JSONResponse: that type's
// generated VisitPostSessionResponse only ever writes the body, with no
// way to also touch a response header, and a cookie is the entire point
// of this response.
type signInResponse struct {
	user  openapi.User
	token string
}

// VisitPostSessionResponse implements openapi.PostSessionResponseObject.
func (r signInResponse) VisitPostSessionResponse(w http.ResponseWriter) error {
	http.SetCookie(w, sessionCookie(r.token, sessionCookieMaxAge))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(r.user); err != nil {
		return fmt.Errorf("encoding sign-in response: %w", err)
	}

	return nil
}

// signOutResponse is DeleteSession's 204: it clears the session cookie,
// whether or not it named a still-valid session. See signInResponse's own
// doc comment for why this is not the generated
// openapi.DeleteSession204Response.
type signOutResponse struct{}

// VisitDeleteSessionResponse implements openapi.DeleteSessionResponseObject.
func (signOutResponse) VisitDeleteSessionResponse(w http.ResponseWriter) error {
	http.SetCookie(w, expiredSessionCookie())
	w.WriteHeader(http.StatusNoContent)

	return nil
}

// expiredSessionCookie is the Set-Cookie a browser needs to drop its
// session cookie immediately: MaxAge<0 (net/http.Cookie's own documented
// way to ask for "Max-Age: 0", i.e. delete now), rather than
// sessionCookie's positive TTL. Secure: true for the same reason
// sessionCookie sets it - see that function's own doc comment.
func expiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

// sessionCookie builds the platform's one, fresh session cookie: HttpOnly
// and SameSite=Lax so a script can never read it and it is never sent
// cross-site (docs/specs/auth.md, A2), valid for maxAge - see
// expiredSessionCookie for the cookie that deletes it.
// http.Cookie.MaxAge takes seconds, not a duration, which is why this
// helper exists rather than every caller converting it inline.
//
// Secure: true unconditionally, not only when the request arrived over
// TLS - gosec's own insecure-cookie check (G124, part of the fixed harness
// policy) requires it to be a literal true, and every place this platform
// runs today (make check's own httptest servers, harness/quality/browser's
// Playwright guard, and a developer on localhost or a loopback address) is
// one net/http/cookiejar and every browser already treat as a secure
// origin regardless of scheme (see cookiejar's own secureMatch). Reaching
// the platform at a non-loopback, non-HTTPS address - the Tailscale
// address docs/plans/workspaces.md's global constraints ask a browser
// check to use - is real, but nothing signs in through a browser yet
// (Task 4); TLS termination is this project's answer when that changes,
// not a weaker cookie.
func sessionCookie(token string, maxAge time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	}
}
