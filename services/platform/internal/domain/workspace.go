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
	// View is how the panel draws its result, beside Args which says what
	// to fetch (docs/specs/dashboard.md, P1). Nil on every panel saved
	// before this slice, and on every panel whose component needs nothing
	// configured - a panel saved with no View still reads back and still
	// draws (AC-P-106).
	View *View
}

// PanelPatch is what UpdatePanel changes about an existing panel - only the
// fields a PATCH request names (docs/specs/dashboard.md, P11, section 6a;
// AC-P-108). Service and OperationID are absent on purpose: a panel's
// operation is fixed once it is made (P13), so there is no field here that
// could even ask to change it.
//
// Title, Args and Component are plain pointers: nil means "leave this
// column alone", a non-nil pointer means "replace it with this" - the usual
// PATCH idiom, and enough for a field with no other state to distinguish. A
// nil Args is "leave alone"; a non-nil, possibly empty, map is "replace
// with these arguments" - the same nil-vs-present test the wire type
// (a Go map unmarshalled from JSON) already gives for free.
//
// View needs a third state Title/Args/Component do not: naming it null has
// to mean something different from leaving it out (section 6a. "remove the
// view" vs "leave it alone" is the one wrinkle in "only the fields the
// request names change"). A single pointer cannot hold three states, so
// View is a pointer to a pointer: nil means untouched, a non-nil pointer to
// a nil *View means "remove the view", and a non-nil pointer to a non-nil
// *View means "replace it with this view" - the same pointer-to-pointer
// idiom PATCH semantics always need once one field can be explicitly
// cleared, chosen here because domain may depend on nothing but the
// standard library (harness/quality/go/golangci.yml, depguard) and so
// cannot reach for github.com/oapi-codegen/nullable.Nullable[T] itself;
// internal/adapter/handler is where a wire UpdatePanelRequest's
// nullable.Nullable[View] becomes one of this type's three states.
type PanelPatch struct {
	Title     *string
	Args      map[string]any
	Component *string
	View      **View
}
