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

// ErrEmptyPassword is returned by EnsureAccount when password is empty,
// for the same reason New refuses an empty adminPassword
// (ErrEmptyAdminPassword): a default password is a way of having none
// while appearing to.
var ErrEmptyPassword = errors.New("the password must not be empty")

// EnsureAccount returns the account named name, creating one with
// password and role first when none exists yet. Unlike New's own seeding
// (which only ever runs once, against an empty table, and only for the
// admin), this checks by name and may be called for any role - it is not
// reachable through any HTTP route (docs/specs/auth.md, section 8: no
// account creation through the UI, and nothing here changes that), only
// through pkg/app's own Config, the platform's composition root - the
// same seam ORCHESTRA_ADMIN_PASSWORD already uses to put the first
// account in place. A caller that already has an account (an operator's
// deployment config, or an acceptance test building the accounts its
// scenario needs) uses this to put a second, third, or non-admin one
// there too, without a network-reachable "anyone can register" endpoint
// ever existing.
func EnsureAccount(ctx context.Context, store *sqlite.Users, name, password string, role domain.Role) (domain.User, error) {
	if password == "" {
		return domain.User{}, ErrEmptyPassword
	}

	if rec, found, err := store.ByName(ctx, name); err != nil {
		return domain.User{}, fmt.Errorf("looking up %q: %w", name, err)
	} else if found {
		return domain.User{ID: rec.ID, Name: rec.Name, Role: rec.Role}, nil
	}

	id, err := store.Create(ctx, name, string(role), hashPassword(password))
	if err != nil {
		return domain.User{}, fmt.Errorf("creating account %q: %w", name, err)
	}

	return domain.User{ID: id, Name: name, Role: role}, nil
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
