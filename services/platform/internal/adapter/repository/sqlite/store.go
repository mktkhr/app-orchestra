// Package sqlite is the platform's implementation of
// usecase.WorkspaceStore: workspaces and their panels, kept in a single
// SQLite file named by ORCHESTRA_DB_PATH (docs/specs/workspaces.md, W3,
// section 6). It uses modernc.org/sqlite, a pure Go driver, so that
// building the platform never needs a C toolchain.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// maxOpenConns keeps every query on one connection. modernc.org/sqlite
// serialises writes at the file level regardless, and one connection
// avoids "database is locked" errors from two goroutines racing to open
// the file - a small, single-process platform has no need of a pool.
const maxOpenConns = 1

// ErrWorkspaceNotFound is returned by AddPanel when the named workspace
// does not exist - the same rule Task 1's handler turns into a 404, and
// the reason AddPanel checks rather than letting a foreign key silently
// orphan the panel.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// Store is a SQLite-backed usecase.WorkspaceStore.
type Store struct {
	db *sql.DB
}

// New opens (creating, if needed) the SQLite file at path and applies the
// embedded schema. The schema is idempotent (every statement is "IF NOT
// EXISTS"), so New is safe to call every time the platform starts - there
// is no migration tool to run first (section 6).
func New(path string) (*Store, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// openDB opens (creating, if needed) the SQLite file at path and applies
// the embedded schema - every table this package knows about, not just the
// ones the caller is about to use, since the schema is idempotent and
// there is only one file (docs/specs/auth.md, section 10: "the accounts
// are rows in the file the workspaces already use"). Shared by every store
// type in this package (Store, Users, Sessions, Permissions) so the "open,
// set the connection limit, apply the schema" sequence is written once.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	db.SetMaxOpenConns(maxOpenConns)

	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("applying schema to %s: %w", path, err)
	}

	if err := ensurePanelsViewColumn(context.Background(), db); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("migrating %s: %w", path, err)
	}

	return db, nil
}

// Close releases the underlying database connection. errors.Join, not a
// plain if, so the return statement - and so this whole function - stays
// exercised by a test whichever way s.db.Close() goes: errors.Join(nil)
// is nil, so a successful close still returns nil.
func (s *Store) Close() error {
	return errors.Join(s.db.Close())
}

// List returns every workspace owned by owner, with its panels in
// position order.
func (s *Store) List(ctx context.Context, owner string) ([]domain.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, owner FROM workspaces WHERE owner = ? ORDER BY id`, owner)
	if err != nil {
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []domain.Workspace

	for rows.Next() {
		var ws domain.Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Owner); err != nil {
			return nil, fmt.Errorf("scanning workspace: %w", err)
		}

		workspaces = append(workspaces, ws)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}

	for i := range workspaces {
		panels, err := s.loadPanels(ctx, workspaces[i].ID)
		if err != nil {
			return nil, err
		}

		workspaces[i].Panels = panels
	}

	return workspaces, nil
}

// Get returns the workspace with the given id, with its panels in
// position order, and whether it was found.
func (s *Store) Get(ctx context.Context, id string) (domain.Workspace, bool, error) {
	var ws domain.Workspace

	row := s.db.QueryRowContext(ctx, `SELECT id, name, owner FROM workspaces WHERE id = ?`, id)
	if err := row.Scan(&ws.ID, &ws.Name, &ws.Owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Workspace{}, false, nil
		}

		return domain.Workspace{}, false, fmt.Errorf("getting workspace %s: %w", id, err)
	}

	panels, err := s.loadPanels(ctx, id)
	if err != nil {
		return domain.Workspace{}, false, err
	}

	ws.Panels = panels

	return ws, true, nil
}

// Create makes a new, empty workspace for owner and returns it.
func (s *Store) Create(ctx context.Context, owner, name string) (domain.Workspace, error) {
	id := newID("ws")

	if _, err := s.db.ExecContext(
		ctx, `INSERT INTO workspaces (id, name, owner) VALUES (?, ?, ?)`, id, name, owner,
	); err != nil {
		return domain.Workspace{}, fmt.Errorf("creating workspace: %w", err)
	}

	return domain.Workspace{ID: id, Name: name, Owner: owner}, nil
}

// Delete removes a workspace and every panel it holds. Deleting a
// workspace that does not exist is not an error: the end state - no such
// workspace - is the same either way.
func (s *Store) Delete(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("deleting workspace %s: %w", id, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM panels WHERE workspace_id = ?`, id); err != nil {
		return rollbackAndWrap(tx, fmt.Errorf("deleting panels of workspace %s: %w", id, err))
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id); err != nil {
		return rollbackAndWrap(tx, fmt.Errorf("deleting workspace %s: %w", id, err))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("deleting workspace %s: %w", id, err)
	}

	return nil
}

// rollbackAndWrap rolls tx back after cause has already decided the
// transaction must not commit, and returns cause - joined with the
// rollback's own error, if it had one. A plain `defer tx.Rollback()` would
// leave that second error unchecked (harness/quality/go/golangci.yml,
// errcheck's check-blank), and this codebase does not silence a linter
// that is telling the truth. errors.Join(cause, nil) is just cause, so the
// common case - rollback succeeds - reads the same as it would unwrapped.
func rollbackAndWrap(tx *sql.Tx, cause error) error {
	return errors.Join(cause, tx.Rollback())
}

// AddPanel appends p to workspaceID's panels and returns the stored panel,
// with its assigned ID and WorkspaceID filled in. It fails with
// ErrWorkspaceNotFound when workspaceID names no workspace, rather than
// silently storing an orphaned panel.
//
// p is a pointer; see usecase.WorkspaceStore.AddPanel's doc comment for
// why.
func (s *Store) AddPanel(ctx context.Context, workspaceID string, p *domain.Panel) (domain.Panel, error) {
	var exists int

	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM workspaces WHERE id = ?`, workspaceID).Scan(&exists)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.Panel{}, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
	case err != nil:
		return domain.Panel{}, fmt.Errorf("checking workspace %s: %w", workspaceID, err)
	}

	id := newID("pnl")

	args := p.Args
	if args == nil {
		args = map[string]any{}
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return domain.Panel{}, fmt.Errorf("encoding panel args: %w", err)
	}

	viewValue, err := marshalView(p.View)
	if err != nil {
		return domain.Panel{}, fmt.Errorf("encoding panel view: %w", err)
	}

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO panels (id, workspace_id, service, operation_id, component, title, args, position, view)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, p.Service, p.OperationID, p.Component, p.Title, string(argsJSON), p.Position, viewValue,
	); err != nil {
		return domain.Panel{}, fmt.Errorf("adding panel to workspace %s: %w", workspaceID, err)
	}

	return domain.Panel{
		ID:          id,
		WorkspaceID: workspaceID,
		Service:     p.Service,
		OperationID: p.OperationID,
		Component:   p.Component,
		Title:       p.Title,
		Args:        args,
		Position:    p.Position,
		View:        p.View,
	}, nil
}

// viewJSON is the wire shape of domain.View, marshaled into the panels
// table's nullable "view" column. It exists because domain may not import
// encoding/json (harness/quality/go/golangci.yml, depguard) - the same
// reason Panel.Args, a plain map, is marshaled here rather than in that
// package - so this adapter is where a domain.View gains and loses its
// JSON tags.
type viewJSON struct {
	Transform *transformJSON `json:"transform,omitempty"`
	Chart     *chartJSON     `json:"chart,omitempty"`
}

type transformJSON struct {
	GroupBy   string           `json:"groupBy"`
	Aggregate domain.Aggregate `json:"aggregate"`
	Field     string           `json:"field,omitempty"`
}

type chartJSON struct {
	Category string           `json:"category"`
	Value    string           `json:"value"`
	Kind     domain.ChartKind `json:"kind"`
}

// marshalView encodes v into a nullable TEXT column value: v == nil
// becomes SQL NULL rather than the literal string "null" (AC-P-106). A
// sql.NullString, not a *string, so an empty view carries no error to
// pair a nil pointer with (golangci-lint's nilnil check forbids a
// function returning (nil, nil) from a pointer-and-error signature).
func marshalView(v *domain.View) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}

	encoded, err := json.Marshal(toViewJSON(v))
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encoding view: %w", err)
	}

	return sql.NullString{String: string(encoded), Valid: true}, nil
}

// unmarshalView decodes a non-NULL panels.view column value into a
// domain.View. Callers check column.Valid themselves (see loadPanels) so
// that this function, unlike marshalView, never has a nil-view case to
// return alongside a nil error - a NULL column and a decoding failure are
// its only two possible reasons not to return a view, and only one of
// them is an error.
func unmarshalView(raw string) (*domain.View, error) {
	var wire viewJSON
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return nil, fmt.Errorf("decoding view: %w", err)
	}

	return fromViewJSON(&wire), nil
}

func toViewJSON(v *domain.View) viewJSON {
	var wire viewJSON

	if v.Transform != nil {
		wire.Transform = &transformJSON{
			GroupBy:   v.Transform.GroupBy,
			Aggregate: v.Transform.Aggregate,
			Field:     v.Transform.Field,
		}
	}

	if v.Chart != nil {
		wire.Chart = &chartJSON{
			Category: v.Chart.Category,
			Value:    v.Chart.Value,
			Kind:     v.Chart.Kind,
		}
	}

	return wire
}

func fromViewJSON(wire *viewJSON) *domain.View {
	v := &domain.View{}

	if wire.Transform != nil {
		v.Transform = &domain.Transform{
			GroupBy:   wire.Transform.GroupBy,
			Aggregate: wire.Transform.Aggregate,
			Field:     wire.Transform.Field,
		}
	}

	if wire.Chart != nil {
		v.Chart = &domain.Chart{
			Category: wire.Chart.Category,
			Value:    wire.Chart.Value,
			Kind:     wire.Chart.Kind,
		}
	}

	return v
}

// DeletePanel removes one panel from a workspace. Deleting a panel that
// does not exist is not an error, for the same reason Delete's is not.
func (s *Store) DeletePanel(ctx context.Context, workspaceID, panelID string) error {
	if _, err := s.db.ExecContext(
		ctx, `DELETE FROM panels WHERE id = ? AND workspace_id = ?`, panelID, workspaceID,
	); err != nil {
		return fmt.Errorf("deleting panel %s from workspace %s: %w", panelID, workspaceID, err)
	}

	return nil
}

// loadPanels returns workspaceID's panels in position order.
func (s *Store) loadPanels(ctx context.Context, workspaceID string) ([]domain.Panel, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, workspace_id, service, operation_id, component, title, args, position, view
		 FROM panels WHERE workspace_id = ? ORDER BY position`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading panels of workspace %s: %w", workspaceID, err)
	}
	defer rows.Close()

	var panels []domain.Panel

	for rows.Next() {
		var (
			p          domain.Panel
			argsJSON   string
			viewColumn sql.NullString
		)

		if err := rows.Scan(
			&p.ID, &p.WorkspaceID, &p.Service, &p.OperationID, &p.Component, &p.Title, &argsJSON, &p.Position,
			&viewColumn,
		); err != nil {
			return nil, fmt.Errorf("scanning panel: %w", err)
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("decoding panel args: %w", err)
		}

		p.Args = args

		// A NULL column - every panel saved before this slice, and every
		// panel with nothing to say here - stays a nil View rather than
		// being decoded at all (AC-P-106).
		if viewColumn.Valid {
			view, err := unmarshalView(viewColumn.String)
			if err != nil {
				return nil, fmt.Errorf("decoding panel view: %w", err)
			}

			p.View = view
		}

		panels = append(panels, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loading panels of workspace %s: %w", workspaceID, err)
	}

	return panels, nil
}
