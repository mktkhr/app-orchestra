package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// dbPath returns a fresh SQLite file path under t.TempDir(): the file
// itself does not exist yet, which is what sqlite.New is expected to
// handle (docs/specs/workspaces.md, section 6).
func dbPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "workspaces.db")
}

func openStore(t *testing.T, path string) *sqlite.Store {
	t.Helper()

	store, err := sqlite.New(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

// TestStoreLifecycle is the scenario docs/plans/workspaces.md, Task 0, Step
// 1 asks for: create a workspace, add two panels, read them back in
// position order, delete one, delete the workspace.
func TestStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "在庫ダッシュボード")
	require.NoError(t, err)
	assert.NotEmpty(t, ws.ID)
	assert.Equal(t, "在庫ダッシュボード", ws.Name)
	assert.Equal(t, "stub-user", ws.Owner)
	assert.Empty(t, ws.Panels)

	// Added out of position order, on purpose: the second panel (position
	// 0) must still come back before the first (position 1).
	second, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   "table",
		Title:       "検品保留の在庫",
		Args:        map[string]any{"status": "quarantined"},
		Position:    0,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, second.ID)
	assert.Equal(t, ws.ID, second.WorkspaceID)

	first, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service:     "attendance",
		OperationID: "ListAttendanceRecords",
		Component:   "table",
		Title:       "本日の出勤",
		Args:        map[string]any{},
		Position:    1,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, first.ID)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 2)
	assert.Equal(t, second.ID, got.Panels[0].ID)
	assert.Equal(t, "検品保留の在庫", got.Panels[0].Title)
	assert.Equal(t, map[string]any{"status": "quarantined"}, got.Panels[0].Args)
	assert.Equal(t, first.ID, got.Panels[1].ID)
	assert.Equal(t, "本日の出勤", got.Panels[1].Title)

	require.NoError(t, store.DeletePanel(ctx, ws.ID, first.ID))

	afterDelete, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, afterDelete.Panels, 1)
	assert.Equal(t, second.ID, afterDelete.Panels[0].ID)

	require.NoError(t, store.Delete(ctx, ws.ID))

	_, ok, err = store.Get(ctx, ws.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestStorePersistsAcrossOpens is half of AC-W-105: a second store opened
// on the same file sees the rows the first one wrote, which is the whole
// point of SQLite over memory (W3).
func TestStorePersistsAcrossOpens(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)

	first := openStore(t, path)

	ws, err := first.Create(ctx, "stub-user", "永続化テスト")
	require.NoError(t, err)

	panel, err := first.AddPanel(ctx, ws.ID, &domain.Panel{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   "table",
		Title:       "在庫一覧",
		Args:        map[string]any{"status": "staged"},
		Position:    0,
	})
	require.NoError(t, err)

	require.NoError(t, first.Close())

	second := openStore(t, path)

	got, ok, err := second.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ws.Name, got.Name)
	assert.Equal(t, ws.Owner, got.Owner)
	require.Len(t, got.Panels, 1)
	assert.Equal(t, panel.ID, got.Panels[0].ID)
	assert.Equal(t, panel.Title, got.Panels[0].Title)
	assert.Equal(t, map[string]any{"status": "staged"}, got.Panels[0].Args)

	listed, err := second.List(ctx, "stub-user")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, ws.ID, listed[0].ID)
}

func TestStoreListFiltersByOwner(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	_, err := store.Create(ctx, "alice", "アリスのワークスペース")
	require.NoError(t, err)
	_, err = store.Create(ctx, "bob", "ボブのワークスペース")
	require.NoError(t, err)

	alices, err := store.List(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, alices, 1)
	assert.Equal(t, "アリスのワークスペース", alices[0].Name)
}

func TestStoreGetReturnsFalseWhenMissing(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	_, ok, err := store.Get(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStoreDeleteMissingWorkspaceIsNotAnError(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	require.NoError(t, store.Delete(ctx, "does-not-exist"))
}

func TestStoreAddPanelToMissingWorkspaceFails(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	_, err := store.AddPanel(ctx, "does-not-exist", &domain.Panel{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   "table",
		Title:       "存在しないワークスペース",
	})
	require.Error(t, err)
}

func TestStoreDeletePanelUnknownIsNotAnError(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	require.NoError(t, store.DeletePanel(ctx, ws.ID, "does-not-exist"))
}

func TestNewFailsOnUnwritablePath(t *testing.T) {
	_, err := sqlite.New(filepath.Join(t.TempDir(), "missing-dir", "workspaces.db"))
	require.Error(t, err)
}

// TestStoreOperationsFailOnClosedStore exercises every method's database
// error path by handing it a store whose connection is already closed -
// the same shape of failure a disk error or a killed connection would
// produce, without needing to fake either.
func TestStoreOperationsFailOnClosedStore(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "クローズ後の店")
	require.NoError(t, err)

	require.NoError(t, store.Close())

	_, err = store.List(ctx, "stub-user")
	require.Error(t, err)

	_, _, err = store.Get(ctx, ws.ID)
	require.Error(t, err)

	_, err = store.Create(ctx, "stub-user", "another")
	require.Error(t, err)

	err = store.Delete(ctx, ws.ID)
	require.Error(t, err)

	_, err = store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.Error(t, err)

	err = store.DeletePanel(ctx, ws.ID, "pnl-x")
	require.Error(t, err)
}

// rawConn opens a second connection straight to path, bypassing the
// Store, so a test can put the database into a shape the Store's own API
// can never produce - a dropped table, or a row with malformed JSON in it
// - to exercise an error path Store cannot otherwise reach. The "sqlite"
// driver is already registered by this package's own blank import.
func rawConn(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func TestStoreAddPanelDefaultsNilArgsToEmptyMap(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	panel, err := store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.NoError(t, err)
	assert.NotNil(t, panel.Args)
	assert.Empty(t, panel.Args)
}

func TestStoreAddPanelFailsToEncodeUnsupportedArgs(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Args:        map[string]any{"callback": func() {}},
	})
	require.Error(t, err)
}

func TestStoreAddPanelFailsWhenPanelsTableIsGone(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `DROP TABLE panels`)
	require.NoError(t, err)

	_, err = store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.Error(t, err)
}

func TestStoreGetFailsWhenPanelsTableIsGone(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `DROP TABLE panels`)
	require.NoError(t, err)

	_, _, err = store.Get(ctx, ws.ID)
	require.Error(t, err)
}

// TestStoreGetFailsOnMalformedPanelColumn plants a panel row whose args or
// view column is not valid JSON - something the Store's own AddPanel can
// never write - to exercise loadPanels' two decode error paths: args'
// own, and unmarshalView's.
func TestStoreGetFailsOnMalformedPanelColumn(t *testing.T) {
	statements := map[string]string{
		"args": `UPDATE panels SET args = 'not-json' WHERE workspace_id = ?`,
		"view": `UPDATE panels SET view = 'not-json' WHERE workspace_id = ?`,
	}

	for column, statement := range statements {
		t.Run(column, func(t *testing.T) {
			ctx := context.Background()
			path := dbPath(t)
			store := openStore(t, path)

			ws, err := store.Create(ctx, "stub-user", "ワークスペース")
			require.NoError(t, err)

			_, err = store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
			require.NoError(t, err)

			_, err = rawConn(t, path).ExecContext(ctx, statement, ws.ID)
			require.NoError(t, err)

			_, _, err = store.Get(ctx, ws.ID)
			require.Error(t, err)
		})
	}
}

func TestStoreListFailsWhenPanelsTableIsGone(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	_, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `DROP TABLE panels`)
	require.NoError(t, err)

	_, err = store.List(ctx, "stub-user")
	require.Error(t, err)
}

// TestStoreDeleteRollsBackWhenDeletingPanelsFails exercises Delete's first
// rollbackAndWrap call: the panels table is gone before the transaction's
// first statement runs, so nothing has been changed yet and the rollback
// itself succeeds.
func TestStoreDeleteRollsBackWhenDeletingPanelsFails(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `DROP TABLE panels`)
	require.NoError(t, err)

	err = store.Delete(ctx, ws.ID)
	require.Error(t, err)
}

// TestStoreDeleteRollsBackWhenDeletingWorkspaceFails exercises Delete's
// second rollbackAndWrap call: panels delete succeeds (there are none),
// then the workspaces table itself is gone.
func TestStoreDeleteRollsBackWhenDeletingWorkspaceFails(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `DROP TABLE workspaces`)
	require.NoError(t, err)

	err = store.Delete(ctx, ws.ID)
	require.Error(t, err)
}

// viewForRoundTrip is a view with both halves set, used to prove a panel's
// view round-trips through the store exactly (docs/plans/dashboard.md,
// Task 2, Step 2).
func viewForRoundTrip() *domain.View {
	return &domain.View{
		Transform: &domain.Transform{
			GroupBy:   "status",
			Aggregate: domain.AggregateCount,
		},
		Chart: &domain.Chart{
			Category: "status",
			Value:    "count",
			Kind:     domain.ChartKindBar,
		},
	}
}

// TestStoreAddPanelRoundTripsView is Task 2 Step 2's first case: a panel
// saved with a view reads back with the same one.
func TestStoreAddPanelRoundTripsView(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	saved, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   "chart",
		Title:       "ステータス別の在庫件数",
		View:        viewForRoundTrip(),
	})
	require.NoError(t, err)
	require.NotNil(t, saved.View)
	assert.Equal(t, viewForRoundTrip(), saved.View)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	require.NotNil(t, got.Panels[0].View)
	assert.Equal(t, viewForRoundTrip(), got.Panels[0].View)
}

// TestStoreAddPanelWithNoViewRoundTripsAsNil is Task 2 Step 2's second
// case: a panel saved without a view reads back with a nil one, not an
// empty-but-present one (AC-P-106).
func TestStoreAddPanelWithNoViewRoundTripsAsNil(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	saved, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "table",
	})
	require.NoError(t, err)
	assert.Nil(t, saved.View)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Nil(t, got.Panels[0].View)
}

// preDashboardSchema is the panels table exactly as it existed before
// docs/plans/dashboard.md's Task 2 added a view column - copied here
// rather than read from schema.sql, which now describes the current
// shape, so this test keeps proving the migration works even after
// schema.sql moves on.
const preDashboardSchema = `
CREATE TABLE workspaces (
	id    TEXT PRIMARY KEY,
	name  TEXT NOT NULL,
	owner TEXT NOT NULL
);

CREATE TABLE panels (
	id           TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	service      TEXT NOT NULL,
	operation_id TEXT NOT NULL,
	component    TEXT NOT NULL,
	title        TEXT NOT NULL,
	args         TEXT NOT NULL,
	position     INTEGER NOT NULL
);
`

// TestNewOpensADatabaseFileWrittenBeforeViewExisted is Task 2 Step 3's own
// requirement: a database file created by the pre-Task-2 schema, with a
// panel already in it, must still open with the current code and that
// panel must still read back - with a nil View, since the column did not
// exist when the row was written (AC-P-106). This is the test that proves
// the migration was actually run against old data, not merely written.
func TestNewOpensADatabaseFileWrittenBeforeViewExisted(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)

	old, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = old.ExecContext(ctx, preDashboardSchema)
	require.NoError(t, err)

	_, err = old.ExecContext(
		ctx,
		`INSERT INTO workspaces (id, name, owner) VALUES (?, ?, ?)`,
		"ws-1", "在庫ダッシュボード", "stub-user",
	)
	require.NoError(t, err)

	_, err = old.ExecContext(
		ctx,
		`INSERT INTO panels (id, workspace_id, service, operation_id, component, title, args, position)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"pnl-1", "ws-1", "inventory", "ListInventoryItems", "table", "検品保留の在庫", `{"status":"quarantined"}`, 0,
	)
	require.NoError(t, err)

	require.NoError(t, old.Close())

	store := openStore(t, path)

	got, ok, err := store.Get(ctx, "ws-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Equal(t, "pnl-1", got.Panels[0].ID)
	assert.Equal(t, "検品保留の在庫", got.Panels[0].Title)
	assert.Equal(t, map[string]any{"status": "quarantined"}, got.Panels[0].Args)
	assert.Nil(t, got.Panels[0].View, "a panel written before the view column existed must read back with a nil view")

	// The migration must also leave the file writable by the current
	// schema going forward: a new panel, saved with a view, still works
	// on the same migrated file.
	saved, err := store.AddPanel(ctx, "ws-1", &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "chart", View: viewForRoundTrip(),
	})
	require.NoError(t, err)
	require.NotNil(t, saved.View)
	assert.Equal(t, viewForRoundTrip(), saved.View)
}
