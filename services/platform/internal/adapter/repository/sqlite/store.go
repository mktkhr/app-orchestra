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
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// panelColumns is the column list loadPanels and loadPanel both select, in
// the order scanPanel expects them.
const panelColumns = "id, workspace_id, service, operation_id, component, title, args, position, width, height, narrow_height, view"

// panelIDAndWorkspaceIDArgs is the two extra query arguments
// UpdatePanel's own WHERE clause always adds, beyond one per SET fragment
// (`id = ?` and `workspace_id = ?`) - named so the slice it sizes does not
// carry an unexplained "+2" (golangci-lint's mnd).
const panelIDAndWorkspaceIDArgs = 2

// maxOpenConns keeps every query on one connection. modernc.org/sqlite
// serialises writes at the file level regardless, and one connection
// avoids two goroutines racing to open a second, independent connection
// to the same file - a small, single-process platform has no need of a
// pool. This alone does not avoid "database is locked": that also needs
// every store on the file to share this one connection (Open, below, is
// how pkg/app.build makes that true - AC-S-102), a busy_timeout so a lock
// held for a moment is waited for rather than reported as a failure, and
// WAL so a reader and a writer are not each other's problem
// (docs/specs/storage.md, S1-S3).
const maxOpenConns = 1

// dsnPragmas is the query string every openDB DSN carries, in the form
// modernc.org/sqlite documents (its "DSN" section): the mattn-compatible
// shorthand keys, one pragma per key, applied to every new physical
// connection the *sql.DB opens - not just the first, which matters here
// only in principle (maxOpenConns keeps this at one connection for the
// process's lifetime) but is how the driver validates the values before
// opening anything, rather than accepting a typo silently.
//
//   - _journal_mode=WAL (S2): a writer no longer excludes every reader,
//     which is the shape of every request this spec is about - a session
//     read that must not queue behind a workspace write.
//   - _busy_timeout=5000 (S3): five seconds is long enough that no honest
//     contention on a single small file reaches a person as an error, and
//     short enough that a lock held that long is a bug, not a timeout to
//     raise further.
//
// Confirmed against the pinned version (modernc.org/sqlite v1.58.0,
// services/platform/go.mod) directly: opening a fresh file with this DSN
// and reading back PRAGMA journal_mode and PRAGMA busy_timeout reports
// "wal" and 5000.
const dsnPragmas = "?_journal_mode=WAL&_busy_timeout=5000"

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
//
// New opens its own connection: a caller that needs it to share a file
// with another store of this package's - pkg/app.build, so a workspace
// read and a session read never contend for two different *sql.DB values
// on the same file (docs/specs/storage.md, AC-S-102) - calls Open itself
// and hands the result to NewFromDB instead.
func New(path string) (*Store, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// NewFromDB builds a Store over db, already open - see New's own doc
// comment, and Open.
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// Open opens (creating, if needed) the SQLite file at path, with the
// pragmas dsnPragmas names, and applies the embedded schema and
// migrations - exactly what New does for its own private connection, but
// returned as a plain *sql.DB so a caller that needs more than one store
// on the same file can open it once and hand that one connection to each
// store's own NewFromDB (NewUsersFromDB, NewSessionsFromDB,
// NewPermissionsFromDB) - see those functions, and pkg/app.build, which is
// the only caller in this codebase that needs to (AC-S-102: "the process
// opens one *sql.DB for its database file, however many stores read it").
func Open(path string) (*sql.DB, error) {
	return openDB(path)
}

// openDB opens (creating, if needed) the SQLite file at path, with the
// pragmas dsnPragmas names, and applies the embedded schema - every table
// this package knows about, not just the ones the caller is about to use,
// since the schema is idempotent and there is only one file
// (docs/specs/auth.md, section 10: "the accounts are rows in the file the
// workspaces already use"). Shared by New and Open.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+dsnPragmas)
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

	if err := ensurePanelsSizeColumns(context.Background(), db); err != nil {
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

	width, height := p.Width, p.Height
	if width == 0 {
		width = domain.DefaultPanelWidth
	}

	if height == 0 {
		height = domain.DefaultPanelHeight
	}

	widthValue := marshalPanelSize(p.Width)
	heightValue := marshalPanelSize(p.Height)
	narrowHeightValue := marshalNullableInt(p.NarrowHeight)

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO panels (id, workspace_id, service, operation_id, component, title, args, position,
		                     width, height, narrow_height, view)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, p.Service, p.OperationID, p.Component, p.Title, string(argsJSON), p.Position,
		widthValue, heightValue, narrowHeightValue, viewValue,
	); err != nil {
		return domain.Panel{}, fmt.Errorf("adding panel to workspace %s: %w", workspaceID, err)
	}

	return domain.Panel{
		ID:           id,
		WorkspaceID:  workspaceID,
		Service:      p.Service,
		OperationID:  p.OperationID,
		Component:    p.Component,
		Title:        p.Title,
		Args:         args,
		Position:     p.Position,
		Width:        width,
		Height:       height,
		NarrowHeight: p.NarrowHeight,
		View:         p.View,
	}, nil
}

// marshalNullableInt encodes a panel's NarrowHeight into the nullable
// narrow_height column: nil - "no narrow height of its own" - becomes SQL
// NULL, the same way marshalView turns a nil View into one. Unlike
// marshalPanelSize, there is no 0-means-unset case to translate: p is
// already a *int, so "the caller said nothing" and "the caller said 0" are
// already distinct by the time this runs, and usecase.Workspaces has
// already clamped whatever non-nil value it holds (docs/specs/layout.md,
// section 5a).
func marshalNullableInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

// marshalPanelSize encodes a panel's Width or Height into the nullable
// column AddPanel writes: 0 - a width or height nothing could mean on
// purpose - becomes SQL NULL rather than the literal 0, the same "not
// really a value" translation marshalView gives a nil View. AddPanel
// computes the value it reports the new panel as actually having
// separately, from the same "0 means unset" rule, defaulting to
// domain.DefaultPanelWidth/DefaultPanelHeight - what a subsequent read of
// the same row will report too, through scanPanel's own NULL-means-default
// handling (docs/plans/layout.md, Task 0, AC-L-104).
//
// Only AddPanel calls this: by the time a create request reaches here,
// usecase.Workspaces.AddPanel has already turned an absent width/height
// into this same default and clamped whatever the caller did send, so 0
// only ever appears when a caller of this store bypasses that usecase
// entirely - as this package's own tests do, on purpose, to prove
// AC-L-104 at the store level.
func marshalPanelSize(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: int64(v), Valid: true}
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

// panelSetClause names the SET fragment and its argument for the one column
// patch names, or ("", nil) when patch does not name it - updatePanelSets
// only appends a fragment onto the query when the second return is true, so
// UpdatePanel writes exactly the columns the caller named (AC-P-108) and
// nothing else.
type panelSetClause struct {
	fragment string
	value    any
}

// updatePanelSets builds the SET fragments and their arguments, in order,
// for the columns patch names - never for one it does not (P11).
func updatePanelSets(patch domain.PanelPatch) ([]panelSetClause, error) {
	var sets []panelSetClause

	if patch.Title != nil {
		sets = append(sets, panelSetClause{"title = ?", *patch.Title})
	}

	if patch.Args != nil {
		argsJSON, err := json.Marshal(patch.Args)
		if err != nil {
			return nil, fmt.Errorf("encoding panel args: %w", err)
		}

		sets = append(sets, panelSetClause{"args = ?", string(argsJSON)})
	}

	if patch.Component != nil {
		sets = append(sets, panelSetClause{"component = ?", *patch.Component})
	}

	if patch.Position != nil {
		sets = append(sets, panelSetClause{"position = ?", *patch.Position})
	}

	if patch.Width != nil {
		sets = append(sets, panelSetClause{"width = ?", *patch.Width})
	}

	if patch.Height != nil {
		sets = append(sets, panelSetClause{"height = ?", *patch.Height})
	}

	if patch.NarrowHeight != nil {
		sets = append(sets, panelSetClause{"narrow_height = ?", *patch.NarrowHeight})
	}

	if patch.View != nil {
		viewValue, err := marshalView(*patch.View)
		if err != nil {
			return nil, fmt.Errorf("encoding panel view: %w", err)
		}

		sets = append(sets, panelSetClause{"view = ?", viewValue})
	}

	return sets, nil
}

// UpdatePanel changes only the columns patch names on one panel
// (AC-P-108) and returns it as it reads afterward, and whether a panel with
// this id existed on this workspace at all - false either when panelID
// names no panel, or when it names one on a different workspace, the same
// answer either way for the reason usecase.Workspaces.UpdatePanel's own
// doc comment gives.
//
// An empty patch - a PATCH naming nothing - still checks the panel exists
// rather than issuing a no-op UPDATE that could never report "not found".
func (s *Store) UpdatePanel(
	ctx context.Context,
	workspaceID, panelID string,
	patch domain.PanelPatch,
) (domain.Panel, bool, error) {
	sets, err := updatePanelSets(patch)
	if err != nil {
		return domain.Panel{}, false, err
	}

	if len(sets) == 0 {
		return s.loadPanel(ctx, workspaceID, panelID)
	}

	fragments := make([]string, len(sets))
	args := make([]any, 0, len(sets)+panelIDAndWorkspaceIDArgs)

	for i, set := range sets {
		fragments[i] = set.fragment
		args = append(args, set.value)
	}

	args = append(args, panelID, workspaceID)

	result, err := s.db.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE panels SET %s WHERE id = ? AND workspace_id = ?`, strings.Join(fragments, ", ")),
		args...,
	)
	if err != nil {
		return domain.Panel{}, false, fmt.Errorf("updating panel %s: %w", panelID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Panel{}, false, fmt.Errorf("updating panel %s: %w", panelID, err)
	}

	if affected == 0 {
		return domain.Panel{}, false, nil
	}

	return s.loadPanel(ctx, workspaceID, panelID)
}

// rowScanner is the part of *sql.Row and *sql.Rows scanPanel needs - one
// method, so it can read a panel's columns from either a single-row or a
// multi-row query without loadPanel and loadPanels each scanning by hand
// (golangci-lint's dupl, harness/quality/go/golangci.yml).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanPanel reads one panel row - the columns panelColumns names, in that
// order - decoding its args and (when the column is non-NULL) its view.
func scanPanel(scanner rowScanner) (domain.Panel, error) {
	var (
		p                  domain.Panel
		argsJSON           string
		widthColumn        sql.NullInt64
		heightColumn       sql.NullInt64
		narrowHeightColumn sql.NullInt64
		viewColumn         sql.NullString
	)

	if err := scanner.Scan(
		&p.ID, &p.WorkspaceID, &p.Service, &p.OperationID, &p.Component, &p.Title, &argsJSON, &p.Position,
		&widthColumn, &heightColumn, &narrowHeightColumn, &viewColumn,
	); err != nil {
		return domain.Panel{}, fmt.Errorf("scanning panel: %w", err)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return domain.Panel{}, fmt.Errorf("decoding panel args: %w", err)
	}

	p.Args = args

	// A NULL width or height column - every panel saved before
	// docs/plans/layout.md's Task 0 added them - reads back as the
	// domain's own default rather than a zero value no grid could draw
	// (AC-L-104).
	p.Width = domain.DefaultPanelWidth
	if widthColumn.Valid {
		p.Width = int(widthColumn.Int64)
	}

	p.Height = domain.DefaultPanelHeight
	if heightColumn.Valid {
		p.Height = int(heightColumn.Int64)
	}

	// A NULL narrow_height column - every panel saved before this field
	// existed, and every panel with no narrow height of its own - stays a
	// nil NarrowHeight rather than being given a default the way Width and
	// Height are: it draws at Height on both breakpoints instead
	// (docs/specs/layout.md, section 5a, AC-L-108).
	if narrowHeightColumn.Valid {
		narrowHeight := int(narrowHeightColumn.Int64)
		p.NarrowHeight = &narrowHeight
	}

	// A NULL column - every panel saved before this slice, and every
	// panel with nothing to say here - stays a nil View rather than
	// being decoded at all (AC-P-106).
	if viewColumn.Valid {
		view, err := unmarshalView(viewColumn.String)
		if err != nil {
			return domain.Panel{}, fmt.Errorf("decoding panel view: %w", err)
		}

		p.View = view
	}

	return p, nil
}

// loadPanels returns workspaceID's panels in position order.
func (s *Store) loadPanels(ctx context.Context, workspaceID string) ([]domain.Panel, error) {
	rows, err := s.db.QueryContext(
		ctx, `SELECT `+panelColumns+` FROM panels WHERE workspace_id = ? ORDER BY position`, workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading panels of workspace %s: %w", workspaceID, err)
	}
	defer rows.Close()

	var panels []domain.Panel

	for rows.Next() {
		p, err := scanPanel(rows)
		if err != nil {
			return nil, err
		}

		panels = append(panels, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loading panels of workspace %s: %w", workspaceID, err)
	}

	return panels, nil
}

// loadPanel returns one panel by id, scoped to workspaceID, and whether it
// was found - false rather than an error when it was not, the same
// not-found-is-a-value rule Store.Get follows.
func (s *Store) loadPanel(ctx context.Context, workspaceID, panelID string) (domain.Panel, bool, error) {
	row := s.db.QueryRowContext(
		ctx, `SELECT `+panelColumns+` FROM panels WHERE id = ? AND workspace_id = ?`, panelID, workspaceID,
	)

	p, err := scanPanel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Panel{}, false, nil
	}

	if err != nil {
		return domain.Panel{}, false, fmt.Errorf("loading panel %s: %w", panelID, err)
	}

	return p, true, nil
}
