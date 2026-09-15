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
	return []Wording{
		v1(), v2Commit(), v3AskOnCollision(), v4CommitAndAsk(), v5ExamplesInTools(), v6UnmatchedFilter(),
	}
}

// Default is v6-unmatched-filter (DECISIONS.md, 2026-09-16, "wording:
// v6-unmatched-filter becomes the default"), which replaced v2-commit
// (docs/plans/wording.md Task 3; DECISIONS.md, 2026-09-15, "wording:
// v2-commit becomes the default"). v6-unmatched-filter closes TODO.md
// item 3 (an unmatched restricting word against an enum parameter
// silently dropped the filter instead of asking): make eval's two
// no-enum-value cases move from 30/30 and 10/10 reject to 0/30 and 0/10
// reject, every accepted outcome an ask_user call on the parameter, no
// other make eval case moved. It costs three points of correct@1 on the
// eval-shortlist corpus (67 vs v2-commit's 68) - accepted because most of
// that loss traces to a pre-existing repetition-loop truncation
// (TODO.md's new "Repetition loop" item), not to the new rule. v1 stays
// in this package as the baseline every candidate - and both decisions -
// is measured against; it is still what
// TestDefaultIsV1ByteIdenticalToTheLiteralsAt5bf5cf8 asserts, by name
// (AC-Q-101), and remains selectable via ORCHESTRA_PLANNER_WORDING=v1;
// v2-commit remains selectable the same way via
// ORCHESTRA_PLANNER_WORDING=v2-commit.
func Default() Wording {
	return v6UnmatchedFilter()
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
