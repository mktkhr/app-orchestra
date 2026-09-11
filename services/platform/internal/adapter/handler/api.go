package handler

import "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"

// API composes every tag's handler into the one type
// openapi.StrictServerInterface needs. Each tag (system, plan, invoke,
// workspaces) is its own small type, tested on its own; API only wires
// them together by embedding, so a method added to any of them is
// promoted here without this file changing.
type API struct {
	*Health
	*Plan
	*Invoke
	*Workspace
}

var _ openapi.StrictServerInterface = API{}

// NewAPI builds the composed strict server interface from its four parts.
func NewAPI(health *Health, plan *Plan, invoke *Invoke, workspace *Workspace) API {
	return API{Health: health, Plan: plan, Invoke: invoke, Workspace: workspace}
}
