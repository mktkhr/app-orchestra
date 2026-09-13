package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// TestWorkspaceHoldsItsPanelsInOrder documents the shape section 3 of
// docs/specs/workspaces.md describes: a Workspace is its panels, and a
// Panel is a saved call plus a title and a position. There is no logic to
// exercise here - domain.Workspace and domain.Panel are pure data, like
// domain.Endpoint - but the shape is worth pinning down as a compile-time
// and field-level contract the rest of the platform builds on.
func TestWorkspaceHoldsItsPanelsInOrder(t *testing.T) {
	ws := domain.Workspace{
		ID:    "ws-1",
		Name:  "在庫ダッシュボード",
		Owner: "stub-user",
		Panels: []domain.Panel{
			{
				ID:          "pnl-1",
				WorkspaceID: "ws-1",
				Service:     "inventory",
				OperationID: "ListInventoryItems",
				Component:   string(domain.ComponentTable),
				Title:       "検品保留の在庫",
				Args:        map[string]any{"status": "quarantined"},
				Position:    0,
			},
		},
	}

	assert.Equal(t, "ws-1", ws.ID)
	assert.Equal(t, "在庫ダッシュボード", ws.Name)
	assert.Equal(t, "stub-user", ws.Owner)
	assert.Len(t, ws.Panels, 1)

	panel := ws.Panels[0]
	assert.Equal(t, "ws-1", panel.WorkspaceID)
	assert.Equal(t, "inventory", panel.Service)
	assert.Equal(t, "ListInventoryItems", panel.OperationID)
	assert.Equal(t, "table", panel.Component)
	assert.Equal(t, "検品保留の在庫", panel.Title)
	assert.Equal(t, map[string]any{"status": "quarantined"}, panel.Args)
	assert.Equal(t, 0, panel.Position)
}

// TestClampPanelWidth pins docs/plans/layout.md Task 0's own examples: a
// width the grid cannot draw is fixed, never rejected - 40 (too wide) comes
// down to MaxPanelWidth, and 0 and -1 (not a width at all) come up to
// MinPanelWidth. A width already in range passes through unchanged.
func TestClampPanelWidth(t *testing.T) {
	tests := map[string]struct {
		width int
		want  int
	}{
		"too wide (40) clamps down to the maximum": {width: 40, want: domain.MaxPanelWidth},
		"zero clamps up to the minimum":            {width: 0, want: domain.MinPanelWidth},
		"negative (-1) clamps up to the minimum":   {width: -1, want: domain.MinPanelWidth},
		"in range passes through unchanged":        {width: 6, want: 6},
		"exactly the minimum passes through":       {width: domain.MinPanelWidth, want: domain.MinPanelWidth},
		"exactly the maximum passes through":       {width: domain.MaxPanelWidth, want: domain.MaxPanelWidth},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.ClampPanelWidth(tt.width))
		})
	}
}

// TestClampPanelHeight is ClampPanelWidth's own case for height - the
// difference being there is no upper bound (section 5: a row's own height
// is not a resource the grid runs out of the way columns are).
func TestClampPanelHeight(t *testing.T) {
	tests := map[string]struct {
		height int
		want   int
	}{
		"zero clamps up to the minimum":          {height: 0, want: domain.MinPanelHeight},
		"negative (-1) clamps up to the minimum": {height: -1, want: domain.MinPanelHeight},
		"in range passes through unchanged":      {height: 5, want: 5},
		"a very tall value is not clamped":       {height: 999, want: 999},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.ClampPanelHeight(tt.height))
		})
	}
}
