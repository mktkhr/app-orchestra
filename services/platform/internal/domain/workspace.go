package domain

// Panel size defaults and bounds (docs/specs/layout.md, section 3): the
// one place either number is decided, so the store (reading a panel saved
// before these columns existed), usecase.Workspaces.AddPanel (creating one
// with neither supplied) and its UpdatePanel (clamping one a caller did
// supply) all read the same values instead of each hardcoding "12" or "1"
// (docs/plans/layout.md, Task 0).
const (
	// DefaultPanelWidth is what a panel with no width at all - never
	// saved with one, or saved before this column existed - reads back
	// as: full width of the single column the grid stacks panels in
	// (docs/specs/layout.md, section 3).
	DefaultPanelWidth = 12
	// DefaultPanelHeight is what a panel with no height at all reads
	// back as: one row tall (docs/specs/layout.md, section 3).
	DefaultPanelHeight = 1
	// MinPanelWidth and MaxPanelWidth bound a panel's span in grid
	// columns (docs/specs/layout.md, section 5: "wide 12 columns").
	MinPanelWidth = 1
	MaxPanelWidth = 12
	// MinPanelHeight bounds a panel's span in grid rows from below.
	// There is no upper bound: a row's own height is not a resource the
	// grid runs out of the way columns are (section 5).
	MinPanelHeight = 1
)

// ClampPanelWidth clamps width to [MinPanelWidth, MaxPanelWidth]. Called by
// usecase.Workspaces at write time (docs/plans/layout.md, Task 0: "a width
// of 40, of 0, or of -1 is not something the grid can draw") - never by the
// browser, and never by rejecting the request: a caller that is not this
// repository's own frontend will send an out-of-range value, and there is
// nothing to tell it that a clamp does not already fix.
func ClampPanelWidth(width int) int {
	return clamp(width, MinPanelWidth, MaxPanelWidth)
}

// ClampPanelHeight clamps height to at least MinPanelHeight, for the same
// reason and at the same call sites as ClampPanelWidth.
func ClampPanelHeight(height int) int {
	if height < MinPanelHeight {
		return MinPanelHeight
	}

	return height
}

func clamp(v, minimum, maximum int) int {
	switch {
	case v < minimum:
		return minimum
	case v > maximum:
		return maximum
	default:
		return v
	}
}

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
	// Position orders a workspace's panels for display (W5), and is now
	// editable via PATCH (docs/specs/layout.md, section 6) - reordering
	// itself (renumbering the panels between one moved and its old spot)
	// is not: a PATCH that sets one panel's position changes only that
	// panel's row.
	Position int
	// Width is the panel's span in grid columns, DefaultPanelWidth when
	// never set (a panel saved before this field existed, or saved with
	// none), always in [MinPanelWidth, MaxPanelWidth] once persisted -
	// usecase.Workspaces clamps it there before it ever reaches a store
	// (docs/specs/layout.md, section 3).
	Width int
	// Height is the panel's span in grid rows, DefaultPanelHeight when
	// never set, always at least MinPanelHeight once persisted, clamped
	// the same way Width is.
	Height int
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
//
// Width, Height and Position are plain "was this field sent at all"
// pointers, the same as Title/Args/Component, not a third pointer-to-
// pointer state like View: an integer has no "explicitly clear it" request
// distinct from "leave it alone" the way naming a view `null` does (there
// is no null width to ask for - only absent-means-unchanged, or a value to
// clamp and store), so nil-means-unchanged is the whole of what these
// three fields need (docs/plans/layout.md, Task 0; see DECISIONS.md).
type PanelPatch struct {
	Title     *string
	Args      map[string]any
	Component *string
	View      **View
	Width     *int
	Height    *int
	Position  *int
}
