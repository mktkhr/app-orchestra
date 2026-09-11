package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// WorkspaceStore persists workspaces and their panels. Implemented by
// internal/adapter/repository/sqlite, which keeps them in the SQLite file
// named by ORCHESTRA_DB_PATH (docs/specs/workspaces.md, W3) - kept as a
// port here so the usecase layer depends on the shape of the storage, not
// on SQLite or the database/sql package, neither of which this layer may
// import (harness/quality/go/golangci.yml, depguard).
type WorkspaceStore interface {
	// List returns every workspace owned by owner, with its panels in
	// position order.
	List(ctx context.Context, owner string) ([]domain.Workspace, error)
	// Get returns the workspace with the given id, with its panels in
	// position order, and whether it was found.
	Get(ctx context.Context, id string) (domain.Workspace, bool, error)
	// Create makes a new, empty workspace for owner and returns it.
	Create(ctx context.Context, owner, name string) (domain.Workspace, error)
	// Delete removes a workspace and every panel it holds.
	Delete(ctx context.Context, id string) error
	// AddPanel appends p to workspaceID's panels and returns the stored
	// panel, with its assigned ID and WorkspaceID filled in.
	//
	// p is a pointer, not the value shown in docs/plans/workspaces.md's
	// Task 0 (`p Panel`), because domain.Panel is 112 bytes:
	// golangci-lint's gocritic hugeParam check (part of the fixed harness
	// policy, see harness/quality/go/golangci.yml) rejects passing it by
	// value - the same reason pkg/app.Config and domain.Endpoint's IsSafe
	// receiver are pointers where the plan or a naive reading would show a
	// value.
	AddPanel(ctx context.Context, workspaceID string, p *domain.Panel) (domain.Panel, error)
	// DeletePanel removes one panel from a workspace.
	DeletePanel(ctx context.Context, workspaceID, panelID string) error
}
