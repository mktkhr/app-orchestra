// Panel view, size (width/height/narrow_height) and pre-existing-column
// migration tests - split out of store_test.go, which passed 1000 lines
// once docs/specs/layout.md section 5a's own narrow-height slice added its
// tests, into harness/quality/go/golangci.yml's own filelen limit
// (guard-filelen). Same package, same helpers (dbPath, openStore,
// newPanelForUpdateTest, rawConn) - only the file is different.
package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

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

// TestStoreAddPanelRoundTripsSize is docs/plans/layout.md Task 0 Step 2's
// first case: a panel saved with an explicit width and height reads back
// with exactly those values, both from AddPanel's own return and from a
// subsequent Get.
func TestStoreAddPanelRoundTripsSize(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	saved, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "table",
		Width: 6, Height: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, 6, saved.Width)
	assert.Equal(t, 3, saved.Height)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Equal(t, 6, got.Panels[0].Width)
	assert.Equal(t, 3, got.Panels[0].Height)
}

// TestStoreAddPanelWithNoSizeRoundTripsAsDefault is Task 0 Step 2's second
// case: a panel saved with neither a width nor a height reads back as
// domain's own default - full width, one row tall (docs/specs/layout.md,
// section 3; AC-L-104) - both from AddPanel's own return and from Get.
// usecase.Workspaces.AddPanel is what actually applies this default on the
// real request path; calling the store directly with a zero Width/Height,
// as here, is what proves the store's own half of that contract - turning
// "unset" into NULL, and NULL back into the default on read - works on its
// own.
func TestStoreAddPanelWithNoSizeRoundTripsAsDefault(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	saved, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "table",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.DefaultPanelWidth, saved.Width)
	assert.Equal(t, domain.DefaultPanelHeight, saved.Height)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Equal(t, domain.DefaultPanelWidth, got.Panels[0].Width)
	assert.Equal(t, domain.DefaultPanelHeight, got.Panels[0].Height)
}

// TestStoreAddPanelRoundTripsNarrowHeight is AC-L-107's own case at the
// store: a panel saved with a NarrowHeight distinct from its Height reads
// back with both intact, from both AddPanel's own return and Get.
func TestStoreAddPanelRoundTripsNarrowHeight(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	narrowHeight := 1

	saved, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "table",
		Height: 3, NarrowHeight: &narrowHeight,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, saved.Height)
	require.NotNil(t, saved.NarrowHeight)
	assert.Equal(t, 1, *saved.NarrowHeight)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Equal(t, 3, got.Panels[0].Height, "changing narrow height must leave height exactly as it was")
	require.NotNil(t, got.Panels[0].NarrowHeight)
	assert.Equal(t, 1, *got.Panels[0].NarrowHeight)
}

// TestStoreAddPanelWithNoNarrowHeightRoundTripsAsNil is AC-L-108's own case
// at the store: a panel saved with no narrow height of its own reads back
// with a nil NarrowHeight - not a default the way Width/Height get one -
// so buildPanelLayout on the frontend is the one place that decides it
// then draws at Height.
func TestStoreAddPanelWithNoNarrowHeightRoundTripsAsNil(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	saved, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "table", Height: 3,
	})
	require.NoError(t, err)
	assert.Nil(t, saved.NarrowHeight)

	got, ok, err := store.Get(ctx, ws.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Nil(t, got.Panels[0].NarrowHeight)
}

// TestStoreUpdatePanelNarrowHeightLeavesHeightAlone is AC-L-107 at the
// store: PATCHing only narrow_height must not touch height.
func TestStoreUpdatePanelNarrowHeightLeavesHeightAlone(t *testing.T) {
	store, ctx, wsID, panel := newPanelForUpdateTest(t)

	updated, found, err := store.UpdatePanel(
		ctx, wsID, panel.ID, domain.PanelPatch{Height: new(3)},
	)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 3, updated.Height)

	narrowHeight := 1

	updated, found, err = store.UpdatePanel(
		ctx, wsID, panel.ID, domain.PanelPatch{NarrowHeight: &narrowHeight},
	)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, updated.NarrowHeight)
	assert.Equal(t, 1, *updated.NarrowHeight)
	assert.Equal(t, 3, updated.Height, "changing narrow height must leave height exactly as it was")
}

// newPanelForUpdateTest opens a fresh store, holding one workspace with one
// panel, for TestStoreUpdatePanelChangesOnlyTheNamedColumn's own subtests -
// each needs its own copy, since each changes a different column of it.
func newPanelForUpdateTest(t *testing.T) (*sqlite.Store, context.Context, string, domain.Panel) {
	t.Helper()

	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	panel, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Component:   "table",
		Title:       "元のタイトル",
		Args:        map[string]any{"status": "quarantined"},
	})
	require.NoError(t, err)

	return store, ctx, ws.ID, panel
}

// TestStoreUpdatePanelChangesOnlyTheNamedColumn is AC-P-108's own case,
// against the real database: title, args, component and view each change
// on their own, and nothing else on the panel moves when only one of them
// is named.
func TestStoreUpdatePanelChangesOnlyTheNamedColumn(t *testing.T) {
	newView := &domain.View{Chart: &domain.Chart{Category: "status", Value: "count", Kind: domain.ChartKindBar}}

	t.Run("title", func(t *testing.T) {
		store, ctx, wsID, panel := newPanelForUpdateTest(t)

		updated, found, err := store.UpdatePanel(ctx, wsID, panel.ID, domain.PanelPatch{Title: new("新しいタイトル")})
		require.NoError(t, err)
		require.True(t, found)

		assert.Equal(t, "新しいタイトル", updated.Title)
		assert.Equal(t, panel.Args, updated.Args)
		assert.Equal(t, panel.Component, updated.Component)
		assert.Nil(t, updated.View)
	})

	t.Run("args", func(t *testing.T) {
		store, ctx, wsID, panel := newPanelForUpdateTest(t)

		newArgs := map[string]any{"status": "staged"}
		updated, found, err := store.UpdatePanel(ctx, wsID, panel.ID, domain.PanelPatch{Args: newArgs})
		require.NoError(t, err)
		require.True(t, found)

		assert.Equal(t, newArgs, updated.Args)
		assert.Equal(t, panel.Title, updated.Title)
		assert.Equal(t, panel.Component, updated.Component)
	})

	t.Run("component", func(t *testing.T) {
		store, ctx, wsID, panel := newPanelForUpdateTest(t)

		updated, found, err := store.UpdatePanel(ctx, wsID, panel.ID, domain.PanelPatch{Component: new("chart")})
		require.NoError(t, err)
		require.True(t, found)

		assert.Equal(t, "chart", updated.Component)
		assert.Equal(t, panel.Title, updated.Title)
		assert.Equal(t, panel.Args, updated.Args)
	})

	t.Run("view", func(t *testing.T) {
		store, ctx, wsID, panel := newPanelForUpdateTest(t)

		updated, found, err := store.UpdatePanel(ctx, wsID, panel.ID, domain.PanelPatch{View: &newView})
		require.NoError(t, err)
		require.True(t, found)

		assert.Equal(t, newView, updated.View)
		assert.Equal(t, panel.Title, updated.Title)
		assert.Equal(t, panel.Args, updated.Args)
		assert.Equal(t, panel.Component, updated.Component)
	})

	// width, height and position each change on their own the same way
	// title/args/component/view already proved above - one table instead
	// of three near-identical t.Run bodies (golangci-lint's dupl).
	sizeCases := map[string]struct {
		patch func() domain.PanelPatch
		check func(t *testing.T, panel, updated domain.Panel)
	}{
		"width": {
			patch: func() domain.PanelPatch { return domain.PanelPatch{Width: new(4)} },
			check: func(t *testing.T, panel, updated domain.Panel) {
				t.Helper()
				assert.Equal(t, 4, updated.Width)
				assert.Equal(t, panel.Height, updated.Height)
				assert.Equal(t, panel.Position, updated.Position)
			},
		},
		"height": {
			patch: func() domain.PanelPatch { return domain.PanelPatch{Height: new(5)} },
			check: func(t *testing.T, panel, updated domain.Panel) {
				t.Helper()
				assert.Equal(t, 5, updated.Height)
				assert.Equal(t, panel.Width, updated.Width)
				assert.Equal(t, panel.Position, updated.Position)
			},
		},
		"position": {
			patch: func() domain.PanelPatch { return domain.PanelPatch{Position: new(7)} },
			check: func(t *testing.T, panel, updated domain.Panel) {
				t.Helper()
				assert.Equal(t, 7, updated.Position)
				assert.Equal(t, panel.Width, updated.Width)
				assert.Equal(t, panel.Height, updated.Height)
			},
		},
	}

	for name, tc := range sizeCases {
		t.Run(name, func(t *testing.T) {
			store, ctx, wsID, panel := newPanelForUpdateTest(t)

			updated, found, err := store.UpdatePanel(ctx, wsID, panel.ID, tc.patch())
			require.NoError(t, err)
			require.True(t, found)

			assert.Equal(t, panel.Title, updated.Title)
			tc.check(t, panel, updated)
		})
	}
}

// TestStoreUpdatePanelPositionDoesNotRenumberOthers is
// docs/specs/layout.md section 6's own requirement: a PATCH that moves one
// panel changes only that panel's position column - the other panels in
// the same workspace keep theirs, rather than the platform renumbering the
// whole list to keep it dense (docs/plans/layout.md, Task 0).
func TestStoreUpdatePanelPositionDoesNotRenumberOthers(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	first, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Position: 0,
	})
	require.NoError(t, err)

	second, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Position: 1,
	})
	require.NoError(t, err)

	third, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Position: 2,
	})
	require.NoError(t, err)

	_, found, err := store.UpdatePanel(ctx, ws.ID, third.ID, domain.PanelPatch{Position: new(0)})
	require.NoError(t, err)
	require.True(t, found)

	unchangedFirst, found, err := store.UpdatePanel(ctx, ws.ID, first.ID, domain.PanelPatch{})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 0, unchangedFirst.Position, "moving the third panel must not touch the first")

	unchangedSecond, found, err := store.UpdatePanel(ctx, ws.ID, second.ID, domain.PanelPatch{})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, unchangedSecond.Position, "moving the third panel must not touch the second")
}

// TestStoreUpdatePanelViewNullVsAbsent is the wrinkle section 6a names:
// naming the view null removes it, and not naming it at all leaves it as it
// was - two different domain.PanelPatch values (a non-nil pointer to a nil
// *View, versus a nil pointer), and two different outcomes.
func TestStoreUpdatePanelViewNullVsAbsent(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	panel, err := store.AddPanel(ctx, ws.ID, &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "chart", View: viewForRoundTrip(),
	})
	require.NoError(t, err)
	require.NotNil(t, panel.View)

	// Absent: PanelPatch.View itself is nil - the view must survive.
	afterAbsent, found, err := store.UpdatePanel(ctx, ws.ID, panel.ID, domain.PanelPatch{Title: new("そのまま")})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, viewForRoundTrip(), afterAbsent.View, "an absent view in the patch must leave the existing one alone")

	// Null: PanelPatch.View points at a nil *View - the view must be removed.
	var cleared *domain.View

	afterNull, found, err := store.UpdatePanel(ctx, ws.ID, panel.ID, domain.PanelPatch{View: &cleared})
	require.NoError(t, err)
	require.True(t, found)
	assert.Nil(t, afterNull.View, "a patch naming the view null must remove it")
}

// TestStoreUpdatePanelUnknownPanelIsNotFound proves UpdatePanel reports
// "not found" - not an error - for a panel id that does not exist on the
// named workspace, and for one that exists on a different one.
func TestStoreUpdatePanelUnknownPanelIsNotFound(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, found, err := store.UpdatePanel(ctx, ws.ID, "does-not-exist", domain.PanelPatch{Title: new("x")})
	require.NoError(t, err)
	assert.False(t, found)

	other, err := store.Create(ctx, "stub-user", "別のワークスペース")
	require.NoError(t, err)

	panel, err := store.AddPanel(ctx, other.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.NoError(t, err)

	_, found, err = store.UpdatePanel(ctx, ws.ID, panel.ID, domain.PanelPatch{Title: new("x")})
	require.NoError(t, err)
	assert.False(t, found, "a panel on a different workspace must not be found either")
}

// TestStoreUpdatePanelEmptyPatchStillChecksExistence proves an empty patch
// - a PATCH naming nothing - is not a silent no-op success over a panel
// that was never there.
func TestStoreUpdatePanelEmptyPatchStillChecksExistence(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	_, found, err := store.UpdatePanel(ctx, ws.ID, "does-not-exist", domain.PanelPatch{})
	require.NoError(t, err)
	assert.False(t, found)

	panel, err := store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.NoError(t, err)

	updated, found, err := store.UpdatePanel(ctx, ws.ID, panel.ID, domain.PanelPatch{})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, panel, updated)
}

// TestStoreUpdatePanelFailsToEncodeUnsupportedArgs mirrors
// TestStoreAddPanelFailsToEncodeUnsupportedArgs for UpdatePanel's own args
// encoding path.
func TestStoreUpdatePanelFailsToEncodeUnsupportedArgs(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, dbPath(t))

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	panel, err := store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.NoError(t, err)

	_, _, err = store.UpdatePanel(
		ctx, ws.ID, panel.ID, domain.PanelPatch{Args: map[string]any{"callback": func() {}}},
	)
	require.Error(t, err)
}

// TestStoreUpdatePanelFailsWhenPanelsTableIsGone exercises UpdatePanel's
// database error path, both for a patch that runs an UPDATE and for an
// empty one that only checks existence.
func TestStoreUpdatePanelFailsWhenPanelsTableIsGone(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)
	store := openStore(t, path)

	ws, err := store.Create(ctx, "stub-user", "ワークスペース")
	require.NoError(t, err)

	panel, err := store.AddPanel(ctx, ws.ID, &domain.Panel{Service: "inventory", OperationID: "ListInventoryItems"})
	require.NoError(t, err)

	_, err = rawConn(t, path).ExecContext(ctx, `DROP TABLE panels`)
	require.NoError(t, err)

	_, _, err = store.UpdatePanel(ctx, ws.ID, panel.ID, domain.PanelPatch{Title: new("x")})
	require.Error(t, err)

	_, _, err = store.UpdatePanel(ctx, ws.ID, panel.ID, domain.PanelPatch{})
	require.Error(t, err)
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
	assert.Equal(t, domain.DefaultPanelWidth, got.Panels[0].Width,
		"a panel written before the width column existed must read back at the default width")
	assert.Equal(t, domain.DefaultPanelHeight, got.Panels[0].Height,
		"a panel written before the height column existed must read back at the default height")

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

// preLayoutSchema is the panels table as it existed after
// docs/plans/dashboard.md's Task 2 (it has a view column) but before
// docs/plans/layout.md's Task 0 (it has neither width nor height) - the
// exact shape of the real database this repository ships against today
// (docs/plans/layout.md, Task 0: "a panel in it has no width and no
// height"), copied here rather than read from schema.sql for the same
// reason preDashboardSchema is.
const preLayoutSchema = `
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
	position     INTEGER NOT NULL,
	view         TEXT
);
`

// TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted is
// docs/plans/layout.md Task 0 Step 2's third case, and the one that
// matters most: a database file written before panels.width and
// panels.height existed - exactly the shape of the real file this
// repository runs against - still opens, and its panel reads back at the
// domain's own default width and height rather than failing to open, or
// reading back as a zero-sized panel nothing could draw (AC-L-104).
// preLayoutSchema also predates panels.narrow_height - the narrow-height
// slice's own migration is the third column ensurePanelsSizeColumns adds
// the same way, so this is the one file already proving a database written
// before any of the three existed still opens, and is extended here (rather
// than duplicated as a fourth test) to check narrow_height reads back nil,
// not some default (docs/specs/layout.md, section 5a, AC-L-108).
func TestNewOpensADatabaseFileWrittenBeforeSizeColumnsExisted(t *testing.T) {
	ctx := context.Background()
	path := dbPath(t)

	old, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = old.ExecContext(ctx, preLayoutSchema)
	require.NoError(t, err)

	_, err = old.ExecContext(
		ctx,
		`INSERT INTO workspaces (id, name, owner) VALUES (?, ?, ?)`,
		"ws-1", "在庫ダッシュボード", "stub-user",
	)
	require.NoError(t, err)

	_, err = old.ExecContext(
		ctx,
		`INSERT INTO panels (id, workspace_id, service, operation_id, component, title, args, position, view)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"pnl-1", "ws-1", "inventory", "ListInventoryItems", "table", "検品保留の在庫", `{"status":"quarantined"}`, 0, nil,
	)
	require.NoError(t, err)

	require.NoError(t, old.Close())

	store := openStore(t, path)

	got, ok, err := store.Get(ctx, "ws-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.Panels, 1)
	assert.Equal(t, "pnl-1", got.Panels[0].ID)
	assert.Equal(t, domain.DefaultPanelWidth, got.Panels[0].Width,
		"a panel written before the width column existed must read back at the default width")
	assert.Equal(t, domain.DefaultPanelHeight, got.Panels[0].Height,
		"a panel written before the height column existed must read back at the default height")
	assert.Nil(t, got.Panels[0].NarrowHeight,
		"a panel written before the narrow_height column existed must read back with a nil narrow height, "+
			"drawing at height on both breakpoints (AC-L-108)")

	// The migration must also leave the file writable going forward: a
	// new panel, saved with an explicit size, still round-trips on the
	// same migrated file.
	narrowHeight := 1

	saved, err := store.AddPanel(ctx, "ws-1", &domain.Panel{
		Service: "inventory", OperationID: "ListInventoryItems", Component: "table",
		Width: 8, Height: 2, NarrowHeight: &narrowHeight,
	})
	require.NoError(t, err)
	assert.Equal(t, 8, saved.Width)
	assert.Equal(t, 2, saved.Height)
	require.NotNil(t, saved.NarrowHeight)
	assert.Equal(t, 1, *saved.NarrowHeight)
}
