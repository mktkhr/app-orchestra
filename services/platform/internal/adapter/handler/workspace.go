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
	UpdatePanel(
		ctx context.Context, user *domain.User, workspaceID, panelID string, patch domain.PanelPatch,
	) (domain.Panel, error)
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

// UpdatePanel implements PATCH /api/workspaces/{id}/panels/{panelId}: only
// the fields the body names change (docs/specs/dashboard.md, P11,
// AC-P-108). Naming an operation the panel's owner may no longer call is a
// 400, and a panel that does not exist, or belongs to somebody else, is a
// 404 - both refused exactly the way addPanelErrorResponse refuses AddPanel
// (AC-P-109).
func (h *Workspace) UpdatePanel(
	ctx context.Context,
	request openapi.UpdatePanelRequestObject,
) (openapi.UpdatePanelResponseObject, error) {
	panel, err := h.workspaces.UpdatePanel(
		ctx, currentUser(ctx), request.Id, request.PanelId, toDomainPanelPatch(request.Body),
	)
	if err != nil {
		if resp, ok := updatePanelErrorResponse(err); ok {
			return resp, nil
		}

		return nil, fmt.Errorf("updating a panel: %w", err)
	}

	return openapi.UpdatePanel200JSONResponse(toAPIPanel(&panel)), nil
}

// updatePanelErrorResponse maps usecase.Workspaces.UpdatePanel's two
// caller-at-fault errors onto their HTTP status, the same way
// addPanelErrorResponse does for AddPanel - see that function's own doc
// comment for why each sentinel means what it does.
func updatePanelErrorResponse(err error) (openapi.UpdatePanelResponseObject, bool) {
	if errors.Is(err, usecase.ErrEndpointNotFound) {
		return openapi.UpdatePanel400JSONResponse{Message: err.Error()}, true
	}

	if errors.Is(err, usecase.ErrWorkspaceNotFound) || errors.Is(err, sqlitestore.ErrWorkspaceNotFound) {
		return openapi.UpdatePanel404JSONResponse{Message: err.Error()}, true
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
		Id:           p.ID,
		WorkspaceId:  p.WorkspaceID,
		Service:      p.Service,
		OperationId:  p.OperationID,
		Args:         args,
		Component:    openapi.Component(p.Component),
		Title:        p.Title,
		Position:     p.Position,
		Width:        p.Width,
		Height:       p.Height,
		NarrowHeight: p.NarrowHeight,
		View:         toAPIView(p.View),
	}
}

// toDomainPanel converts a CreatePanelRequest into the domain.Panel
// usecase.Workspaces.AddPanel takes. workspaceID is not part of the
// request body - it comes from the path - so it is filled in here rather
// than left for the usecase to thread through separately.
//
// An absent Width/Height crosses over as a plain 0 - domain.Panel's own
// fields are plain ints, so this is the point a *int's "the caller said
// nothing" collapses into the same zero value a genuinely-sent 0 would
// have meant, and there is no width or height a caller could mean by
// "zero" anyway. usecase.Workspaces.AddPanel and, beneath it,
// sqlite.Store.AddPanel are what turn that 0 into
// domain.DefaultPanelWidth/DefaultPanelHeight (docs/plans/layout.md, Task
// 0) - not this handler, which stays a plain wire-to-domain translation.
func toDomainPanel(workspaceID string, body *openapi.CreatePanelRequest) *domain.Panel {
	var width, height int

	if body.Width != nil {
		width = *body.Width
	}

	if body.Height != nil {
		height = *body.Height
	}

	return &domain.Panel{
		WorkspaceID: workspaceID,
		Service:     body.Service,
		OperationID: body.OperationId,
		Component:   string(body.Component),
		Title:       body.Title,
		Args:        body.Args,
		Width:       width,
		Height:      height,
		// body.NarrowHeight is threaded straight across, unlike
		// Width/Height above: it is already a *int on the wire, and nil
		// here means the same thing it means everywhere else this field
		// travels - "no narrow height of its own" - rather than a zero
		// collapsed from "the caller said nothing" (see domain.Panel's own
		// doc comment).
		NarrowHeight: body.NarrowHeight,
		View:         toDomainView(body.View),
	}
}

// toDomainPanelPatch converts an UpdatePanelRequest into the
// domain.PanelPatch usecase.Workspaces.UpdatePanel takes. Title, Args,
// Component, Position, Width, Height and NarrowHeight are plain "was this
// field sent at all" pointers, straight off the generated (already-optional)
// wire type - Position, Width, Height and NarrowHeight need no third state
// the way View does (see
// domain.PanelPatch's own doc comment), so body.Position/Width/Height are
// assigned across unchanged. View is the one field with a
// third state to translate: body.View is a
// github.com/oapi-codegen/nullable.Nullable[openapi.View] (see
// openapi.yaml's UpdatePanelRequest.view, `x-go-type`), and its
// IsSpecified()/IsNull() answer exactly the two questions domain.PanelPatch's
// **domain.View needs - "was it in the body at all" and, if so, "was it
// null" - so this is the one place that nullable.Nullable type has to be
// understood at all; everything downstream sees only **domain.View.
func toDomainPanelPatch(body *openapi.UpdatePanelRequest) domain.PanelPatch {
	patch := domain.PanelPatch{Title: body.Title}

	if body.Args != nil {
		patch.Args = *body.Args
	}

	if body.Component != nil {
		component := string(*body.Component)
		patch.Component = &component
	}

	patch.Position = body.Position
	patch.Width = body.Width
	patch.Height = body.Height
	patch.NarrowHeight = body.NarrowHeight

	if body.View.IsSpecified() {
		var view *domain.View
		if !body.View.IsNull() {
			wire := body.View.MustGet()
			view = toDomainView(&wire)
		}

		patch.View = &view
	}

	return patch
}

// toAPIView converts a domain.View into the wire View, and nil into nil -
// a panel saved before this slice, or with nothing to configure, carries
// no view either way (AC-P-106).
func toAPIView(v *domain.View) *openapi.View {
	if v == nil {
		return nil
	}

	out := &openapi.View{}

	if v.Transform != nil {
		out.Transform = &struct {
			Aggregate openapi.ViewTransformAggregate `json:"aggregate"`
			Field     *string                        `json:"field,omitempty"`
			GroupBy   string                         `json:"groupBy"`
		}{
			GroupBy:   v.Transform.GroupBy,
			Aggregate: openapi.ViewTransformAggregate(v.Transform.Aggregate),
			Field:     emptyToNilString(v.Transform.Field),
		}
	}

	if v.Chart != nil {
		out.Chart = &struct {
			Category string                `json:"category"`
			Kind     openapi.ViewChartKind `json:"kind"`
			Value    string                `json:"value"`
		}{
			Category: v.Chart.Category,
			Value:    v.Chart.Value,
			Kind:     openapi.ViewChartKind(v.Chart.Kind),
		}
	}

	return out
}

// toDomainView converts the wire View into a domain.View, and nil into
// nil, the same way toAPIView goes the other direction.
func toDomainView(v *openapi.View) *domain.View {
	if v == nil {
		return nil
	}

	out := &domain.View{}

	if v.Transform != nil {
		field := ""
		if v.Transform.Field != nil {
			field = *v.Transform.Field
		}

		out.Transform = &domain.Transform{
			GroupBy:   v.Transform.GroupBy,
			Aggregate: domain.Aggregate(v.Transform.Aggregate),
			Field:     field,
		}
	}

	if v.Chart != nil {
		out.Chart = &domain.Chart{
			Category: v.Chart.Category,
			Value:    v.Chart.Value,
			Kind:     domain.ChartKind(v.Chart.Kind),
		}
	}

	return out
}

// emptyToNilString returns nil for an empty string and a pointer to s
// otherwise - Transform.Field is absent for `count`, and an empty string
// pointer would round-trip as a present-but-blank field instead.
func emptyToNilString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
