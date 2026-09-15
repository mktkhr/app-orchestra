// Package wording holds the toolcall planner's words - the system
// message, the three built-in tools' descriptions and how a catalogue
// tool's own description is built from its endpoint - as a named,
// versioned set (docs/specs/wording.md, section 3). A wording changes
// strings only: it never touches the planner's mechanics (temperature,
// max_tokens, tool schemas, narrowing, K - docs/plans/wording.md, Global
// constraints).
//
// internal/adapter/planner/toolcall applies whichever set it is built
// with (toolcall.WithWording); the jsonmode planner is out of scope
// (docs/plans/wording.md, Task 1, Step 5) and keeps reading
// internal/usecase's own tool descriptions directly.
package wording

// Wording is one named set of the toolcall planner's words.
type Wording struct {
	// Name selects this set via ORCHESTRA_PLANNER_WORDING
	// (internal/infra/config).
	Name string
	// SystemPrompt is the toolcall planner's system message.
	SystemPrompt string
	// AskUser overrides usecase.AskUserTool's own Description for the
	// toolcall planner only - the usecase's own text stays exactly as it
	// is, for the jsonmode planner and any other caller of
	// usecase.AskUserTool that does not go through this adapter
	// (docs/plans/wording.md, Task 1, Step 2).
	AskUser string
	// ListCapabilities overrides usecase.ListCapabilitiesTool's own
	// Description, the same way AskUser overrides usecase.AskUserTool's.
	ListCapabilities string
	// ProposePanel overrides usecase.ProposePanelTool's own Description,
	// the same way AskUser overrides usecase.AskUserTool's.
	ProposePanel string
	// CatalogueTool renders one catalogue operation's tool description
	// from its endpoint's own summary (usecase.Tool.Description) and its
	// x-orchestra-examples (domain.Endpoint.Examples, carried as
	// usecase.Tool.Examples - data only, the usecase layer renders no
	// prompt text itself). Today (v1) this is exactly the summary,
	// unchanged.
	CatalogueTool func(summary string, examples []string) string
}

// all lists every named set this package declares, in the order Names
// reports them: declared order, not sorted - v1 first, since it is
// Default and every candidate's own doc comment is written as a delta
// from it.
func all() []Wording {
	return []Wording{v1(), v2Commit(), v3AskOnCollision(), v4CommitAndAsk(), v5ExamplesInTools()}
}

// Default is v1: the text in the product today, byte for byte
// (docs/specs/wording.md, Q1; AC-Q-101), and stays the default until a
// recorded decision changes it (docs/specs/wording.md, section 5).
func Default() Wording {
	return v1()
}

// ByName looks a set up by Wording.Name, reporting false when name is not
// one all declares - the shape ORCHESTRA_PLANNER_WORDING's validation
// (internal/infra/config) and pkg/app both need for an unknown name to
// fail startup (AC-Q-102).
func ByName(name string) (Wording, bool) {
	for _, w := range all() {
		if w.Name == name {
			return w, true
		}
	}

	return Wording{}, false
}

// Names lists every set's Name, in all's declared order - what an unknown
// ORCHESTRA_PLANNER_WORDING's startup error lists.
func Names() []string {
	ws := all()

	names := make([]string, len(ws))
	for i, w := range ws {
		names[i] = w.Name
	}

	return names
}
