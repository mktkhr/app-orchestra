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

// TestStoreGetFailsOnMalformedPanelArgs plants a panel row whose args
// column is not valid JSON - something the Store's own AddPanel can never
// write - to exercise loadPanels' decode error path.
func TestStoreGetFailsOnMalformedPanelArgs(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, err = store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `UPDATE panels SET args = 'not-json' WHERE workspace_id = ?`, ws.ID)
	require.NoError(t, err)

	_, _, err = store.Get(ctx, ws.ID)
	require.Error(t, err)
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
