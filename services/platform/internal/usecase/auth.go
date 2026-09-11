package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// Authenticator checks a name and password against the platform's
// accounts and answers with the matching domain.User, or false when
// either is wrong (docs/specs/auth.md, A1). Implemented by
// internal/adapter/auth/local, which reads local accounts from the
// platform's own SQLite file and checks an argon2id hash - kept as a port
// here so this layer depends on the shape of "who is this", not on how a
// password is hashed, which is exactly the seam docs/requirements.md
// section 5 asks identity to have: an OIDC adapter would implement the
// same port.
type Authenticator interface {
	Authenticate(ctx context.Context, name, password string) (domain.User, bool, error)
}

// SessionStore keeps the opaque token a signed-in person's browser carries
// (docs/specs/auth.md, A2). Implemented by internal/adapter/repository/sqlite,
// alongside WorkspaceStore, in the same SQLite file - kept as a port here
// for the same reason WorkspaceStore is: this layer depends on the shape
// of the storage, not on SQLite or database/sql, neither of which it may
// import (harness/quality/go/golangci.yml, depguard).
type SessionStore interface {
	// Create starts a new session for userID and returns its token. The
	// token is random and opaque, and carries an expiry the store keeps
	// alongside it - never readable from the token itself.
	Create(ctx context.Context, userID string) (token string, err error)
	// User returns the account a still-valid token belongs to, and whether
	// one was found - false for an unknown or expired token, the same
	// answer either way, since the difference is not this port's to tell a
	// caller apart.
	User(ctx context.Context, token string) (domain.User, bool, error)
	// Delete ends a session. Deleting one that does not exist is not an
	// error: the end state - no such session - is the same either way.
	Delete(ctx context.Context, token string) error
}

// PermissionStore keeps the rows that say what a person may call
// (docs/specs/auth.md, section 4, A3). A row says this person may call
// this operation; there is no deny, because there is nothing to override.
type PermissionStore interface {
	// For returns every permission held by userID.
	For(ctx context.Context, userID string) ([]domain.Permission, error)
	// Set replaces every permission userID holds with permissions, wholesale:
	// a grant screen writes the whole set it shows, not a diff against what
	// was there before.
	Set(ctx context.Context, userID string, permissions []domain.Permission) error
}
