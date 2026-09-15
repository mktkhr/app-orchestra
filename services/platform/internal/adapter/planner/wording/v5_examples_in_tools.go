package wording

import "strings"

// v5Name is "v5-examples-in-tools".
const v5Name = "v5-examples-in-tools"

// v5CatalogueTool targets what docs/specs/wording.md section 3 raises:
// today a catalogue tool's description is only its endpoint's summary -
// the written examples that lifted retrieval (x-orchestra-examples,
// domain.Endpoint.Examples) never reach the planner at all. v5 keeps
// v1's words everywhere else and appends every example, quoted, after
// the summary: "\n例: 「…」「…」", in the endpoint's own declared order.
// A summary with no examples is returned unchanged - the same case v1
// always returns.
func v5CatalogueTool(summary string, examples []string) string {
	if len(examples) == 0 {
		return summary
	}

	var b strings.Builder

	b.WriteString(summary)
	b.WriteString("\n例: ")

	for _, example := range examples {
		b.WriteString("「")
		b.WriteString(example)
		b.WriteString("」")
	}

	return b.String()
}

// v5ExamplesInTools is v1's words exactly, everywhere except
// CatalogueTool (see v5CatalogueTool's own doc comment).
func v5ExamplesInTools() Wording {
	return Wording{
		Name:             v5Name,
		SystemPrompt:     v1SystemPrompt,
		AskUser:          v1AskUser,
		ListCapabilities: v1ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v5CatalogueTool,
	}
}
