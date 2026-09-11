package usecase

import (
	"context"
	"fmt"

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

// stubOwner is every workspace's owner until authentication exists (W6,
// docs/specs/workspaces.md). A fixed string rather than an empty one, so
// that "no owner yet" and "the stub owner" are never confused with each
// other once a real one exists. This is the one place the value is
// written: Workspaces is the only caller of WorkspaceStore's
// owner-scoped methods, so wiring authentication in later is a change to
// this constant alone, not to every call site that reads or writes a
// workspace.
const stubOwner = "stub-user"

// Workspaces is the usecase over WorkspaceStore: every workspace it reads
// or writes belongs to stubOwner (W6), and a panel may only be added when
// it names an operation the catalogue exposes - the same rule
// Orchestrator.Invoke applies via catalog.Find, for the same reason
// (docs/specs/workspaces.md, section 5). A panel that could be saved
// unexposed would fail every time its workspace was opened, since opening
// a workspace re-runs each panel through /api/invoke (W2) - so the check
// belongs here, at write time, not left to the moment it fails.
type Workspaces struct {
	store   WorkspaceStore
	catalog domain.Catalog
}

// NewWorkspaces builds a Workspaces usecase over store, validating panels
// against catalog.
func NewWorkspaces(store WorkspaceStore, catalog domain.Catalog) *Workspaces {
	return &Workspaces{store: store, catalog: catalog}
}

// List returns every workspace of the stub owner.
func (w *Workspaces) List(ctx context.Context) ([]domain.Workspace, error) {
	workspaces, err := w.store.List(ctx, stubOwner)
	if err != nil {
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}

	return workspaces, nil
}

// Get returns the workspace with the given id, and whether it was found.
func (w *Workspaces) Get(ctx context.Context, id string) (domain.Workspace, bool, error) {
	workspace, found, err := w.store.Get(ctx, id)
	if err != nil {
		return domain.Workspace{}, false, fmt.Errorf("reading workspace %s: %w", id, err)
	}

	return workspace, found, nil
}

// Create makes a new, empty workspace for the stub owner.
func (w *Workspaces) Create(ctx context.Context, name string) (domain.Workspace, error) {
	workspace, err := w.store.Create(ctx, stubOwner, name)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("creating workspace %q: %w", name, err)
	}

	return workspace, nil
}

// Delete removes a workspace and every panel it holds.
func (w *Workspaces) Delete(ctx context.Context, id string) error {
	if err := w.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting workspace %s: %w", id, err)
	}

	return nil
}

// AddPanel saves p as a new panel on workspaceID, rejecting it with
// ErrEndpointNotFound when the catalogue does not expose (p.Service,
// p.OperationID) - the same sentinel Invoke returns for the same reason,
// so a handler maps both the same way. An empty p.Title is replaced with
// p.OperationID before it reaches the store: a panel is drawn in a card
// with its title as the header (section 8), and an empty header reads as
// a broken card rather than an unnamed one.
//
// p is a pointer for the same gocritic hugeParam reason as
// WorkspaceStore.AddPanel's own parameter (domain.Panel is 112 bytes; see
// that method's doc comment).
func (w *Workspaces) AddPanel(ctx context.Context, workspaceID string, p *domain.Panel) (domain.Panel, error) {
	if _, ok := w.catalog.Find(p.Service, p.OperationID); !ok {
		return domain.Panel{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, p.Service, p.OperationID)
	}

	panel := *p
	if panel.Title == "" {
		panel.Title = panel.OperationID
	}

	saved, err := w.store.AddPanel(ctx, workspaceID, &panel)
	if err != nil {
		return domain.Panel{}, fmt.Errorf("adding a panel to workspace %s: %w", workspaceID, err)
	}

	return saved, nil
}

// DeletePanel removes one panel from a workspace.
func (w *Workspaces) DeletePanel(ctx context.Context, workspaceID, panelID string) error {
	if err := w.store.DeletePanel(ctx, workspaceID, panelID); err != nil {
		return fmt.Errorf("deleting panel %s from workspace %s: %w", panelID, workspaceID, err)
	}

	return nil
}
