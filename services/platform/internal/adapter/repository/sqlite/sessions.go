package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// sessionTTL is how long a session stays valid after it is created. Fixed
// rather than configurable: nothing in docs/specs/auth.md asks for a
// tunable lifetime, and a session that outlives a working day is already
// generous for a single-tenant PoC.
const sessionTTL = 24 * time.Hour

// Sessions is a SQLite-backed usecase.SessionStore. A distinct type from
// Store, in the same package - see Users' doc comment for why: Store
// already has a Delete method of its own (for workspaces), and Go does not
// allow two methods of the same name on the same receiver.
type Sessions struct {
	db *sql.DB
}

// NewSessions opens (creating, if needed) the SQLite file at path and
// applies the embedded schema, the same as Store.New - see that function's
// own doc comment for when a caller should use NewSessionsFromDB (Open)
// instead.
func NewSessions(path string) (*Sessions, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	return &Sessions{db: db}, nil
}

// NewSessionsFromDB builds a Sessions over db, already open - see
// Store.NewFromDB and Open.
func NewSessionsFromDB(db *sql.DB) *Sessions {
	return &Sessions{db: db}
}

// Close releases the underlying database connection.
func (s *Sessions) Close() error {
	return errors.Join(s.db.Close())
}

// Create starts a new session for userID and returns its token: random and
// opaque, from the same newID this package already uses for workspaces and
// panels (docs/specs/auth.md, A2), carrying no "sess-" meaning beyond
// telling it apart from a workspace or panel id at a glance.
func (s *Sessions) Create(ctx context.Context, userID string) (string, error) {
	token := newID("sess")
	expiresAt := time.Now().Add(sessionTTL)

	if _, err := s.db.ExecContext(
		ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expiresAt.Unix(),
	); err != nil {
		return "", fmt.Errorf("creating a session: %w", err)
	}

	return token, nil
}

// User returns the account a still-valid token belongs to, and whether one
// was found. An unknown token and an expired one answer the same way -
// false - since neither is this store's business to tell a caller apart.
func (s *Sessions) User(ctx context.Context, token string) (domain.User, bool, error) {
	var (
		u    domain.User
		role string
	)

	row := s.db.QueryRowContext(
		ctx,
		`SELECT users.id, users.name, users.role
		 FROM sessions JOIN users ON users.id = sessions.user_id
		 WHERE sessions.token = ? AND sessions.expires_at > ?`,
		token, time.Now().Unix(),
	)
	if err := row.Scan(&u.ID, &u.Name, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, false, nil
		}

		return domain.User{}, false, fmt.Errorf("looking up session: %w", err)
	}

	u.Role = domain.Role(role)

	return u, true, nil
}

// Delete ends a session. Deleting one that does not exist is not an error,
// for the same reason Store.Delete's is not.
func (s *Sessions) Delete(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	return nil
}
