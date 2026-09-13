package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// Permissions is a SQLite-backed usecase.PermissionStore. A distinct type
// from Store, in the same package - see Users' doc comment for why.
type Permissions struct {
	db *sql.DB
}

// NewPermissions opens (creating, if needed) the SQLite file at path and
// applies the embedded schema, the same as Store.New - see that function's
// own doc comment for when a caller should use NewPermissionsFromDB (Open)
// instead.
func NewPermissions(path string) (*Permissions, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	return &Permissions{db: db}, nil
}

// NewPermissionsFromDB builds a Permissions over db, already open - see
// Store.NewFromDB and Open.
func NewPermissionsFromDB(db *sql.DB) *Permissions {
	return &Permissions{db: db}
}

// Close releases the underlying database connection.
func (p *Permissions) Close() error {
	return errors.Join(p.db.Close())
}

// For returns every permission held by userID.
func (p *Permissions) For(ctx context.Context, userID string) ([]domain.Permission, error) {
	rows, err := p.db.QueryContext(
		ctx,
		`SELECT service, operation_id FROM permissions WHERE user_id = ? ORDER BY service, operation_id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading permissions of %s: %w", userID, err)
	}
	defer rows.Close()

	var permissions []domain.Permission

	for rows.Next() {
		var perm domain.Permission
		if err := rows.Scan(&perm.Service, &perm.OperationID); err != nil {
			return nil, fmt.Errorf("scanning permission: %w", err)
		}

		permissions = append(permissions, perm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading permissions of %s: %w", userID, err)
	}

	return permissions, nil
}

// Set replaces every permission userID holds with permissions, wholesale:
// delete what is there, then insert what was asked for, in one
// transaction - the same replace-wholesale shape a grant screen writes
// (docs/plans/auth.md, Task 0, Step 1).
func (p *Permissions) Set(ctx context.Context, userID string, permissions []domain.Permission) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("setting permissions of %s: %w", userID, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM permissions WHERE user_id = ?`, userID); err != nil {
		return rollbackAndWrap(tx, fmt.Errorf("clearing permissions of %s: %w", userID, err))
	}

	for _, perm := range permissions {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO permissions (user_id, service, operation_id) VALUES (?, ?, ?)`,
			userID, perm.Service, perm.OperationID,
		); err != nil {
			return rollbackAndWrap(tx, fmt.Errorf("granting %s/%s to %s: %w", perm.Service, perm.OperationID, userID, err))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("setting permissions of %s: %w", userID, err)
	}

	return nil
}
