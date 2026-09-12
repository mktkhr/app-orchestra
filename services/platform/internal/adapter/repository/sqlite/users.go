package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// UserRecord is one stored account, hash included. It never crosses the
// usecase boundary - domain.User has no such field - but
// internal/adapter/auth/local needs the hash to check a password against,
// which is why Users.ByName hands it back this shape rather than
// domain.User.
type UserRecord struct {
	ID   string
	Name string
	Role domain.Role
	Hash string
}

// Users is a SQLite-backed store of accounts: the storage half of
// internal/adapter/auth/local's Authenticator (docs/specs/auth.md,
// section 3). A distinct type from Store, in the same package, rather
// than more methods on Store itself: usecase.SessionStore's Delete and
// Store's own workspace-deleting Delete would otherwise be two methods of
// the same name on the same receiver, which Go does not allow.
type Users struct {
	db *sql.DB
}

// NewUsers opens (creating, if needed) the SQLite file at path and applies
// the embedded schema, the same as Store.New - see openDB.
func NewUsers(path string) (*Users, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	return &Users{db: db}, nil
}

// Close releases the underlying database connection.
func (u *Users) Close() error {
	return errors.Join(u.db.Close())
}

// ByName returns the account named name, hash included, and whether it
// was found.
func (u *Users) ByName(ctx context.Context, name string) (UserRecord, bool, error) {
	var (
		rec  UserRecord
		role string
	)

	row := u.db.QueryRowContext(ctx, `SELECT id, name, role, password_hash FROM users WHERE name = ?`, name)
	if err := row.Scan(&rec.ID, &rec.Name, &role, &rec.Hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserRecord{}, false, nil
		}

		return UserRecord{}, false, fmt.Errorf("looking up user %q: %w", name, err)
	}

	rec.Role = domain.Role(role)

	return rec, true, nil
}

// List returns every account (docs/specs/auth.md, section 6: GET
// /api/users), in name order - no password hash, the same reason
// domain.User itself carries none.
func (u *Users) List(ctx context.Context) ([]domain.User, error) {
	rows, err := u.db.QueryContext(ctx, `SELECT id, name, role FROM users ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	defer rows.Close()

	var users []domain.User

	for rows.Next() {
		var (
			user domain.User
			role string
		)

		if err := rows.Scan(&user.ID, &user.Name, &role); err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}

		user.Role = domain.Role(role)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	return users, nil
}

// Create inserts a new account named name, with role and hash as given,
// and returns its assigned id. Unlike SeedAdminIfNone, this does not check
// whether the table is already populated - it is the lower-level primitive
// internal/adapter/auth/local.EnsureAccount uses to create an account with
// any role, once it has already decided (by name) that none exists yet.
// A duplicate name is rejected by the users table's own UNIQUE constraint
// (schema.sql), surfaced here as a plain error.
func (u *Users) Create(ctx context.Context, name, role, hash string) (string, error) {
	id := newID("usr")

	if _, err := u.db.ExecContext(
		ctx,
		`INSERT INTO users (id, name, role, password_hash) VALUES (?, ?, ?, ?)`,
		id, name, role, hash,
	); err != nil {
		return "", fmt.Errorf("creating account %q: %w", name, err)
	}

	return id, nil
}

// SeedAdminIfNone inserts a fresh admin account named name, with hash as
// its password hash, but only when the users table is currently empty -
// so a platform started against a database that already has accounts
// never overwrites what is there (docs/specs/auth.md, section 3;
// docs/plans/auth.md, Task 0, Step 3). Returns whether it seeded one.
func (u *Users) SeedAdminIfNone(ctx context.Context, name, hash string) (bool, error) {
	var count int

	if err := u.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, fmt.Errorf("counting users: %w", err)
	}

	if count > 0 {
		return false, nil
	}

	id := newID("usr")

	if _, err := u.db.ExecContext(
		ctx,
		`INSERT INTO users (id, name, role, password_hash) VALUES (?, ?, ?, ?)`,
		id, name, string(domain.RoleAdmin), hash,
	); err != nil {
		return false, fmt.Errorf("seeding the admin account: %w", err)
	}

	return true, nil
}
