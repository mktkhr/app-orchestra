package handler

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// userContextKey and sessionTokenContextKey are unexported so nothing
// outside this package can read or forge either value directly - only
// through WithUser/currentUser and WithSessionToken/sessionToken.
type (
	userContextKey         struct{}
	sessionTokenContextKey struct{}
)

// WithUser returns ctx carrying user: the shape
// internal/infra/httpserver's session middleware sets, once it has
// resolved a request's cookie, so currentUser can read it back out
// (docs/plans/auth.md, Task 2, Step 3). user is nil for a request that
// carried no valid session - only GetSession ever sees that case, since
// the middleware answers 401 itself for every other route before a
// handler runs.
func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// currentUser resolves the person a request is running as, and is the one
// call site every handler in this package goes through to get one
// (Plan.PostPlan, Invoke.PostInvoke, Workspace's own methods, and
// Session.GetSession). Reads back whatever internal/infra/httpserver's
// session middleware put on ctx with WithUser - nil when nobody is signed
// in.
func currentUser(ctx context.Context) *domain.User {
	if user, ok := ctx.Value(userContextKey{}).(*domain.User); ok {
		return user
	}

	return nil
}

// WithSessionToken returns ctx carrying the raw value of the request's
// session cookie, or "" when it carried none. Set by the same session
// middleware that calls WithUser, so DeleteSession can end the exact
// session a browser named without parsing a cookie itself - the
// middleware already did, once, for every request.
func WithSessionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessionTokenContextKey{}, token)
}

// sessionToken reads back what WithSessionToken set, or "" when nothing
// did.
func sessionToken(ctx context.Context) string {
	if token, ok := ctx.Value(sessionTokenContextKey{}).(string); ok {
		return token
	}

	return ""
}
