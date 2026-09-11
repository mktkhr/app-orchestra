package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	sqlitestore "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/repository/sqlite"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// workspaces is what Workspace needs from the usecase layer: satisfied by
// *usecase.Workspaces. An interface here, rather than the concrete type,
// keeps this handler's test doubles simple (see planner in plan.go).
type workspaces interface {
	List(ctx context.Context, user *domain.User) ([]domain.Workspace, error)
	Get(ctx context.Context, user *domain.User, id string) (domain.Workspace, bool, error)
	Create(ctx context.Context, user *domain.User, name string) (domain.Workspace, error)
	Delete(ctx context.Context, user *domain.User, id string) error
	AddPanel(ctx context.Context, user *domain.User, workspaceID string, p *domain.Panel) (domain.Panel, error)
	DeletePanel(ctx context.Context, user *domain.User, workspaceID, panelID string) error
}

// Workspace implements the "workspaces" tag of the generated strict server
// interface: every /api/workspaces route.
type Workspace struct {
	workspaces workspaces
}

// NewWorkspace builds the /api/workspaces handler over ws.
func NewWorkspace(ws workspaces) *Workspace {
	return &Workspace{workspaces: ws}
}

// ListWorkspaces implements GET /api/workspaces: no panel data, only a
// count - see WorkspaceSummary in the contract.
func (h *Workspace) ListWorkspaces(
	ctx context.Context,
	_ openapi.ListWorkspacesRequestObject,
) (openapi.ListWorkspacesResponseObject, error) {
	list, err := h.workspaces.List(ctx, currentUser(ctx))
	if err != nil {
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}

	out := make(openapi.ListWorkspaces200JSONResponse, len(list))
	for i := range list {
		out[i] = toAPIWorkspaceSummary(&list[i])
	}

	return out, nil
}

// CreateWorkspace implements POST /api/workspaces.
func (h *Workspace) CreateWorkspace(
	ctx context.Context,
	request openapi.CreateWorkspaceRequestObject,
) (openapi.CreateWorkspaceResponseObject, error) {
	ws, err := h.workspaces.Create(ctx, currentUser(ctx), request.Body.Name)
	if err != nil {
		return nil, fmt.Errorf("creating a workspace: %w", err)
	}

	return openapi.CreateWorkspace201JSONResponse{Id: ws.ID, Name: ws.Name}, nil
}

// GetWorkspace implements GET /api/workspaces/{id}: the workspace and its
// panels, in position order. No panel carries its data - the browser
// posts each one to /api/invoke itself (docs/specs/workspaces.md, section
// 5).
func (h *Workspace) GetWorkspace(
	ctx context.Context,
	request openapi.GetWorkspaceRequestObject,
) (openapi.GetWorkspaceResponseObject, error) {
	ws, ok, err := h.workspaces.Get(ctx, currentUser(ctx), request.Id)
	if err != nil {
		return nil, fmt.Errorf("reading a workspace: %w", err)
	}

	if !ok {
		return openapi.GetWorkspace404JSONResponse{Message: "workspace not found: " + request.Id}, nil
	}

	return openapi.GetWorkspace200JSONResponse(toAPIWorkspace(&ws)), nil
}

// DeleteWorkspace implements DELETE /api/workspaces/{id}. Deleting a
// workspace that does not exist is not an error: the end state - no such
// workspace - is the same either way (usecase.WorkspaceStore.Delete).
func (h *Workspace) DeleteWorkspace(
	ctx context.Context,
	request openapi.DeleteWorkspaceRequestObject,
) (openapi.DeleteWorkspaceResponseObject, error) {
	if err := h.workspaces.Delete(ctx, currentUser(ctx), request.Id); err != nil {
		return nil, fmt.Errorf("deleting a workspace: %w", err)
	}

	return openapi.DeleteWorkspace204Response{}, nil
}

// AddPanel implements POST /api/workspaces/{id}/panels: saves a call as a
// new panel. An operation the catalogue does not expose is rejected as a
// 400 - the same rule /api/invoke follows via catalog.Find, applied here
// at save time instead of at the first, doomed-to-fail open
// (docs/plans/workspaces.md, Task 1). An unknown workspace is a 404.
func (h *Workspace) AddPanel(
	ctx context.Context,
	request openapi.AddPanelRequestObject,
) (openapi.AddPanelResponseObject, error) {
	panel, err := h.workspaces.AddPanel(ctx, currentUser(ctx), request.Id, toDomainPanel(request.Id, request.Body))
	if err != nil {
		if resp, ok := addPanelErrorResponse(err); ok {
			return resp, nil
		}

		return nil, fmt.Errorf("adding a panel: %w", err)
	}

	return openapi.AddPanel201JSONResponse(toAPIPanel(&panel)), nil
}

// addPanelErrorResponse maps the two AddPanel errors the caller is at
// fault for onto their HTTP status: naming an operation the catalogue
// does not expose (400, usecase.ErrEndpointNotFound - the same sentinel
// Invoke returns, see invoke.go's invokeErrorResponse) and naming a
// workspace that does not exist, or that belongs to somebody else (404,
// usecase.ErrWorkspaceNotFound - the usecase's own ownership check, see
// Workspaces.AddPanel - or sqlitestore.ErrWorkspaceNotFound, still
// possible from the store's own foreign-key check underneath it). Its
// second return value is false for anything else - a genuine storage
// failure - which the caller reports as an error instead, the same way
// ListWorkspaces, GetWorkspace, DeleteWorkspace and DeletePanel already
// do for their own store errors: openapi.NewStrictHandler renders any
// returned error as a 500, so there is no bespoke AddPanel500JSONResponse
// to build here.
func addPanelErrorResponse(err error) (openapi.AddPanelResponseObject, bool) {
	if errors.Is(err, usecase.ErrEndpointNotFound) {
		return openapi.AddPanel400JSONResponse{Message: err.Error()}, true
	}

	if errors.Is(err, usecase.ErrWorkspaceNotFound) || errors.Is(err, sqlitestore.ErrWorkspaceNotFound) {
		return openapi.AddPanel404JSONResponse{Message: err.Error()}, true
	}

	return nil, false
}

// DeletePanel implements DELETE /api/workspaces/{id}/panels/{panelId}.
// Deleting a panel that does not exist is not an error, for the same
// reason DeleteWorkspace's is not.
func (h *Workspace) DeletePanel(
	ctx context.Context,
	request openapi.DeletePanelRequestObject,
) (openapi.DeletePanelResponseObject, error) {
	if err := h.workspaces.DeletePanel(ctx, currentUser(ctx), request.Id, request.PanelId); err != nil {
		return nil, fmt.Errorf("deleting a panel: %w", err)
	}

	return openapi.DeletePanel204Response{}, nil
}

// toAPIWorkspaceSummary converts one domain.Workspace into the wire
// WorkspaceSummary: no panel data, only how many there are - see
// ListWorkspaces.
func toAPIWorkspaceSummary(ws *domain.Workspace) openapi.WorkspaceSummary {
	return openapi.WorkspaceSummary{Id: ws.ID, Name: ws.Name, PanelCount: len(ws.Panels)}
}

// toAPIWorkspace converts one domain.Workspace, with its panels, into the
// wire Workspace.
func toAPIWorkspace(ws *domain.Workspace) openapi.Workspace {
	panels := make([]openapi.Panel, len(ws.Panels))
	for i := range ws.Panels {
		panels[i] = toAPIPanel(&ws.Panels[i])
	}

	return openapi.Workspace{Id: ws.ID, Name: ws.Name, Panels: panels}
}

// toAPIPanel converts one domain.Panel into the wire Panel.
func toAPIPanel(p *domain.Panel) openapi.Panel {
	args := p.Args
	if args == nil {
		args = map[string]any{}
	}

	return openapi.Panel{
		Id:          p.ID,
		WorkspaceId: p.WorkspaceID,
		Service:     p.Service,
		OperationId: p.OperationID,
		Args:        args,
		Component:   openapi.Component(p.Component),
		Title:       p.Title,
		Position:    p.Position,
	}
}

// toDomainPanel converts a CreatePanelRequest into the domain.Panel
// usecase.Workspaces.AddPanel takes. workspaceID is not part of the
// request body - it comes from the path - so it is filled in here rather
// than left for the usecase to thread through separately.
func toDomainPanel(workspaceID string, body *openapi.CreatePanelRequest) *domain.Panel {
	return &domain.Panel{
		WorkspaceID: workspaceID,
		Service:     body.Service,
		OperationID: body.OperationId,
		Component:   string(body.Component),
		Title:       body.Title,
		Args:        body.Args,
	}
}
