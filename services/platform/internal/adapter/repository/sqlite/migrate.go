package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ensurePanelsViewColumn adds panels.view when it is missing, so a
// database file written before docs/plans/dashboard.md's Task 2 still
// opens and its panels still read back (AC-P-106).
//
// schemaSQL's own "CREATE TABLE IF NOT EXISTS" does nothing for a column
// on a table that already exists, and modernc.org/sqlite's parser does not
// accept SQLite's own "ALTER TABLE ... ADD COLUMN IF NOT EXISTS" (it
// rejects the trailing "IF NOT EXISTS" with a syntax error even though the
// server's sqlite_version() is 3.53.4, well past the 3.35 that upstream
// SQLite added it in), so the idempotence has to be done here: look at
// panels' own columns first, and only ALTER TABLE when "view" is not among
// them. Running it twice, or against a file that already has the column,
// is then exactly as safe as every "IF NOT EXISTS" statement in
// schema.sql - it just checks first instead of saying so in the SQL.
func ensurePanelsViewColumn(ctx context.Context, db *sql.DB) error {
	has, err := panelsHasViewColumn(ctx, db)
	if err != nil {
		return fmt.Errorf("checking panels.view: %w", err)
	}

	if has {
		return nil
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE panels ADD COLUMN view TEXT`); err != nil {
		return fmt.Errorf("adding panels.view: %w", err)
	}

	return nil
}

// panelsHasViewColumn reports whether the panels table already has a
// "view" column, read from SQLite's own pragma_table_info table-valued
// function - the introspection this package's migration logic uses
// instead of an "IF NOT EXISTS" clause the driver does not accept on
// ALTER TABLE ADD COLUMN. Written the same "SELECT 1 ... Scan" shape as
// AddPanel's own existence check, just against pragma_table_info instead
// of a table this package owns.
func panelsHasViewColumn(ctx context.Context, db *sql.DB) (bool, error) {
	var exists int

	err := db.QueryRowContext(
		ctx, `SELECT 1 FROM pragma_table_info('panels') WHERE name = 'view'`,
	).Scan(&exists)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("reading panels' columns: %w", err)
	default:
		return true, nil
	}
}
