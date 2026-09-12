package usecase

import (
	"context"
	"errors"
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

// ErrWorkspaceNotFound is returned by AddPanel when workspaceID names a
// workspace that either does not exist or does not belong to the calling
// user - the same answer either way (docs/specs/auth.md, section 7: "one
// belonging to somebody else is not found rather than forbidden - a 403
// tells you a thing exists"). Named in this package, not borrowed from
// internal/adapter/repository/sqlite, because usecase may not import an
// adapter package (layer order, harness/quality/go/golangci.yml,
// depguard); internal/adapter/handler maps both this and the store's own
// sqlite.ErrWorkspaceNotFound onto the same 404.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// Workspaces is the usecase over WorkspaceStore: every workspace it reads
// or writes belongs to the user passed to it (docs/specs/auth.md, section
// 7), and a panel may only be added when it names an operation the
// catalogue exposes - the same rule Orchestrator.Invoke applies via
// catalog.Find, for the same reason (docs/specs/workspaces.md, section 5).
// A panel that could be saved unexposed would fail every time its
// workspace was opened, since opening a workspace re-runs each panel
// through /api/invoke (W2) - so the check belongs here, at write time, not
// left to the moment it fails.
//
// Workspaces once held no PermissionStore, on the reasoning that a
// workspace's own access rule is ownership, and a panel naming an
// operation its owner may no longer call could be left to fail where it
// is actually invoked - Orchestrator.Invoke, through the narrowed
// catalogue (docs/specs/auth.md, section 7; AC-W-106). Building a panel
// changed that: docs/specs/dashboard.md's AC-P-107 requires that a person
// may not build a panel over an operation they may not call "through this
// endpoint any more than through /api/invoke" - so AddPanel now narrows
// the catalogue the same way catalogFor does there, and needs the same
// PermissionStore to do it (see DECISIONS.md).
type Workspaces struct {
	store       WorkspaceStore
	catalog     domain.Catalog
	permissions PermissionStore
}

// NewWorkspaces builds a Workspaces usecase over store, validating panels
// against catalog narrowed by permissions.
func NewWorkspaces(store WorkspaceStore, catalog domain.Catalog, permissions PermissionStore) *Workspaces {
	return &Workspaces{store: store, catalog: catalog, permissions: permissions}
}

// List returns every workspace user owns.
func (w *Workspaces) List(ctx context.Context, user *domain.User) ([]domain.Workspace, error) {
	workspaces, err := w.store.List(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}

	return workspaces, nil
}

// Get returns the workspace with the given id, and whether it was found -
// false both when no such workspace exists and when one does but belongs
// to somebody other than user (docs/specs/auth.md, section 7): the same
// answer either way, since a 403 would tell the caller the workspace
// exists.
func (w *Workspaces) Get(ctx context.Context, user *domain.User, id string) (domain.Workspace, bool, error) {
	workspace, found, err := w.store.Get(ctx, id)
	if err != nil {
		return domain.Workspace{}, false, fmt.Errorf("reading workspace %s: %w", id, err)
	}

	if !found || workspace.Owner != user.ID {
		return domain.Workspace{}, false, nil
	}

	return workspace, true, nil
}

// Create makes a new, empty workspace for user.
func (w *Workspaces) Create(ctx context.Context, user *domain.User, name string) (domain.Workspace, error) {
	workspace, err := w.store.Create(ctx, user.ID, name)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("creating workspace %q: %w", name, err)
	}

	return workspace, nil
}

// Delete removes a workspace and every panel it holds. Deleting a
// workspace that does not exist, or that belongs to somebody other than
// user, is not an error: the end state - the workspace gone from user's
// own view - is indistinguishable either way, and the same
// not-found-rather-than-forbidden rule Get follows applies here too
// (docs/specs/auth.md, section 7).
func (w *Workspaces) Delete(ctx context.Context, user *domain.User, id string) error {
	workspace, found, err := w.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("reading workspace %s: %w", id, err)
	}

	if !found || workspace.Owner != user.ID {
		return nil
	}

	if err := w.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting workspace %s: %w", id, err)
	}

	return nil
}

// AddPanel saves p as a new panel on workspaceID, rejecting it with
// ErrEndpointNotFound when catalogFor(ctx, user) does not expose
// (p.Service, p.OperationID) - the same sentinel Invoke returns for the
// same reason, so a handler maps both the same way, and the same error
// whether the operation does not exist at all or user's own permissions
// simply do not reach it (docs/specs/dashboard.md, AC-P-107: a 400 never
// tells anybody what exists) - and with ErrWorkspaceNotFound when
// workspaceID does not exist or belongs to somebody other than user
// (docs/specs/auth.md, section 7). An empty p.Title is replaced with
// p.OperationID before it reaches the store: a panel is drawn in a card
// with its title as the header (section 8), and an empty header reads as
// a broken card rather than an unnamed one.
//
// p is a pointer for the same gocritic hugeParam reason as
// WorkspaceStore.AddPanel's own parameter (domain.Panel is 112 bytes; see
// that method's doc comment).
func (w *Workspaces) AddPanel(
	ctx context.Context,
	user *domain.User,
	workspaceID string,
	p *domain.Panel,
) (domain.Panel, error) {
	catalog, err := w.catalogFor(ctx, user)
	if err != nil {
		return domain.Panel{}, err
	}

	if _, ok := catalog.Find(p.Service, p.OperationID); !ok {
		return domain.Panel{}, fmt.Errorf("%w: %s/%s", ErrEndpointNotFound, p.Service, p.OperationID)
	}

	workspace, found, err := w.store.Get(ctx, workspaceID)
	if err != nil {
		return domain.Panel{}, fmt.Errorf("reading workspace %s: %w", workspaceID, err)
	}

	if !found || workspace.Owner != user.ID {
		return domain.Panel{}, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, workspaceID)
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

// DeletePanel removes one panel from a workspace. Deleting from a
// workspace that does not exist, or that belongs to somebody other than
// user, is not an error, for the same reason Delete's is not.
func (w *Workspaces) DeletePanel(ctx context.Context, user *domain.User, workspaceID, panelID string) error {
	workspace, found, err := w.store.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("reading workspace %s: %w", workspaceID, err)
	}

	if !found || workspace.Owner != user.ID {
		return nil
	}

	if err := w.store.DeletePanel(ctx, workspaceID, panelID); err != nil {
		return fmt.Errorf("deleting panel %s from workspace %s: %w", panelID, workspaceID, err)
	}

	return nil
}

// catalogFor narrows w.catalog to what user may call: the whole catalogue
// for an admin, or domain.Catalog.For(permissions) for anybody else - the
// same rule Orchestrator.catalogFor applies, for the same reason
// (docs/specs/auth.md, section 5): the operation AddPanel will accept and
// the operation /api/invoke will later run must be read from the same
// narrowed value, or a panel could be built over something its owner
// cannot actually call.
func (w *Workspaces) catalogFor(ctx context.Context, user *domain.User) (domain.Catalog, error) {
	if user.Role == domain.RoleAdmin {
		return w.catalog, nil
	}

	permissions, err := w.permissions.For(ctx, user.ID)
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("reading permissions for %s: %w", user.ID, err)
	}

	return w.catalog.For(permissions), nil
}
