package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// newStubAdmin is who every request runs as when NewRouter is built with
// no SessionUsers store - see requireSession's own doc comment for who
// that is and why. A function rather than a package-level value (this
// codebase's gochecknoglobals policy, harness/quality/go/golangci.yml):
// each request gets its own *domain.User, the same id and role the fixed
// stub in adapter/handler/user.go answered with before this file existed.
func newStubAdmin() *domain.User {
	return &domain.User{ID: "stub-admin", Role: domain.RoleAdmin}
}

// isUnauthenticatedPath reports whether path is exempt from requireSession's
// 401.
//
//   - /api/health answers whether the process is up, which is not a
//     question about a person (docs/specs/auth.md, section 6).
//   - /api/session is exempt at the middleware level for all three of its
//     methods: POST is how a person gets a session in the first place, so
//     blocking it without one would make signing in impossible; GET and
//     DELETE report 401 (GET) or do nothing (DELETE) themselves, in
//     adapter/handler/session.go, when nobody is signed in - the same
//     answer either way, so there is nothing for this middleware to add
//     by blocking them first.
func isUnauthenticatedPath(path string) bool {
	return path == "/api/health" || path == "/api/session"
}

// SessionUsers is what requireSession needs from usecase.SessionStore:
// just the read. A separate, smaller interface here for the same reason
// workspaces in adapter/handler/workspace.go is one - and exported,
// unlike that one, because pkg/app.build must name this type: the
// function-value assignment `build(cfg, httpserver.NewRouter)` requires
// NewRouter's own parameter type to be spelled out identically at both
// ends.
type SessionUsers interface {
	User(ctx context.Context, token string) (domain.User, bool, error)
}

// requireSession resolves the request's session cookie into a user - once,
// here, for every request - and sets it on the request's context with
// handler.WithUser so every handler downstream reads the same value back
// with currentUser (docs/plans/auth.md, Task 2, Step 3). Also sets the raw
// cookie value with handler.WithSessionToken, for DeleteSession's own use.
//
// A request naming no valid session is answered 401 directly, without
// reaching next, unless its path is in unauthenticatedPaths.
//
// store may be nil: pkg/app builds NewRouter this way for its own
// pre-auth tests - any Config that leaves DBPath empty, the same case
// pkg/app.newWorkspaceHandler's own doc comment describes for the
// workspace store. Every request then runs as newStubAdmin's user and
// nothing is ever refused, matching the fixed stub
// adapter/handler/user.go's currentUser answered with before this file
// existed.
func requireSession(store SessionUsers, next http.Handler) http.Handler {
	if store == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := handler.WithUser(r.Context(), newStubAdmin())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := sessionCookieValue(r)

		ctx := handler.WithSessionToken(r.Context(), token)

		var user *domain.User

		if token != "" {
			resolved, ok, err := store.User(ctx, token)
			if err != nil {
				writeSessionError(w, http.StatusInternalServerError, fmt.Sprintf("resolving session: %v", err))

				return
			}

			if ok {
				user = &resolved
			}
		}

		ctx = handler.WithUser(ctx, user)

		if user == nil && !isUnauthenticatedPath(r.URL.Path) {
			writeSessionError(w, http.StatusUnauthorized, "sign in required")

			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sessionCookieValue returns r's session cookie value, or "" when it
// carried none.
func sessionCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(handler.SessionCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// writeSessionError writes an openapi.ErrorResponse body with status -
// the same shape every other handler's own error responses use, so a 401
// from this middleware looks like one from any other endpoint.
func writeSessionError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(openapi.ErrorResponse{Message: message}); err != nil {
		// The status is already written; there is nowhere left to report
		// an encoding failure to but the connection itself, and net/http
		// already drops it there once headers are sent.
		return
	}
}
