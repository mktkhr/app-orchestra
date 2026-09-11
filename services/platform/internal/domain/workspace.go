package domain

// Workspace is a person's saved collection of panels: a place to keep an
// answer worth looking at again (docs/specs/workspaces.md, W1, W3).
type Workspace struct {
	ID     string
	Name   string
	Owner  string
	Panels []Panel
}

// Panel is a saved call: everything a plan result's source and component
// already carry, plus a title and a position. Opening a workspace re-runs
// each panel's call through /api/invoke rather than storing its answer
// (docs/specs/workspaces.md, W2) - the fields below are the whole of what
// a panel is (section 3).
type Panel struct {
	ID          string
	WorkspaceID string
	Service     string
	OperationID string
	Component   string
	Title       string
	// Args is the panel's call arguments. A map[string]any, like
	// Endpoint's schema-less counterparts elsewhere in this package,
	// because domain may depend on nothing but the standard library
	// (harness/quality/go/golangci.yml, depguard) and so cannot describe
	// this with a JSON library's own types.
	Args map[string]any
	// Position orders a workspace's panels for display (W5). The column
	// that would record a drag exists; the editing does not (section 9).
	Position int
}
