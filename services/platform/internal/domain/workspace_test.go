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
