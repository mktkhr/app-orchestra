package handler

import "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"

// API composes every tag's handler into the one type
// openapi.StrictServerInterface needs. Each tag (system, session, plan,
// invoke, workspaces, users) is its own small type, tested on its own; API
// only wires them together by embedding, so a method added to any of them
// is promoted here without this file changing.
type API struct {
	*Health
	*Session
	*Plan
	*Invoke
	*Workspace
	*Users
}

var _ openapi.StrictServerInterface = API{}

// NewAPI builds the composed strict server interface from its six parts.
func NewAPI(health *Health, session *Session, plan *Plan, invoke *Invoke, workspace *Workspace, users *Users) API {
	return API{Health: health, Session: session, Plan: plan, Invoke: invoke, Workspace: workspace, Users: users}
}
