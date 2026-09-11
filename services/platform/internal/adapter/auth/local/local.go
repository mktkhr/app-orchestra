// Package local implements usecase.Authenticator over the platform's own
// SQLite file: argon2id over the users table (docs/specs/auth.md, A1,
// section 3). An OIDC adapter would implement the same port - this is one
// implementation of it, not the abstraction itself.
package local

import (
	"context"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// ErrEmptyAdminPassword is returned by New when adminPassword is empty:
// a default password would be a way of having none while appearing to
// (docs/specs/auth.md, section 3), the same reasoning
// internal/infra/config.ErrMissingDBPath already applies to
// ORCHESTRA_DB_PATH.
var ErrEmptyAdminPassword = errors.New("the admin password must not be empty")

// Authenticator is a usecase.Authenticator backed by sqlite.Users.
type Authenticator struct {
	store *sqlite.Users
}

// New builds an Authenticator over store, seeding a first admin named
// adminName from adminPassword when the users table is empty
// (docs/specs/auth.md, section 3; docs/plans/auth.md, Task 0, Step 3) -
// once, since sqlite.Users.SeedAdminIfNone does nothing when an account
// already exists. adminPassword must not be empty.
func New(ctx context.Context, store *sqlite.Users, adminName, adminPassword string) (*Authenticator, error) {
	if adminPassword == "" {
		return nil, ErrEmptyAdminPassword
	}

	if _, err := store.SeedAdminIfNone(ctx, adminName, hashPassword(adminPassword)); err != nil {
		return nil, fmt.Errorf("seeding the admin account: %w", err)
	}

	return &Authenticator{store: store}, nil
}

// Authenticate checks name and password against the stored accounts and
// answers with the matching domain.User, or false when either is wrong -
// the same answer either way, so a caller can never tell "no such account"
// from "wrong password" (docs/specs/auth.md, A1).
func (a *Authenticator) Authenticate(ctx context.Context, name, password string) (domain.User, bool, error) {
	rec, found, err := a.store.ByName(ctx, name)
	if err != nil {
		return domain.User{}, false, fmt.Errorf("looking up %q: %w", name, err)
	}

	if !found {
		return domain.User{}, false, nil
	}

	ok, err := verifyPassword(password, rec.Hash)
	if err != nil {
		return domain.User{}, false, fmt.Errorf("verifying the password for %q: %w", name, err)
	}

	if !ok {
		return domain.User{}, false, nil
	}

	return domain.User{ID: rec.ID, Name: rec.Name, Role: rec.Role}, true, nil
}
