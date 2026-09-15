package wording

// v1Name is "v1".
const v1Name = "v1"

// v1SystemPrompt is internal/adapter/planner/toolcall.systemPrompt as of
// 5bf5cf8, moved here unchanged (docs/plans/wording.md, Task 1, Step 2):
// it tells the model how to use the catalogue's tools - call one, call
// list_capabilities for a question about what can be done at all, call
// ask_user when an enum value is ambiguous, or answer nothing when
// nothing fits.
const v1SystemPrompt = "You are given a set of tools, one per operation of a catalogue of " +
	"internal services, plus ask_user, list_capabilities and propose_panel. Read the user's " +
	"question, in Japanese, and either call exactly one tool that answers it, call " +
	"list_capabilities when the question asks what can be done rather than asking to do " +
	"something, call ask_user when a parameter's value cannot be told from the question, call " +
	"propose_panel when the question asks to put something on the workspace's screen rather than " +
	"asking to look something up, or call no tool at all when nothing in the catalogue answers " +
	"the question."

// v1AskUser is internal/usecase.askUserDescription as of 5bf5cf8, moved
// here unchanged: when to reach for ask_user instead of one of the
// catalogue's own tools.
const v1AskUser = "Call this ONLY when the question does not tell you which value to use for " +
	"a parameter that declares a fixed set of allowed values (an enum), and you need the person to pick " +
	"one from that set. Do NOT call this for a free-text parameter (for example a name) that has no " +
	"declared set of values - there is nothing to pick from, so this tool cannot help; leave that " +
	"parameter out of your call instead."

// v1ListCapabilities is internal/usecase.listCapabilitiesDescription as of
// 5bf5cf8, moved here unchanged: when to reach for list_capabilities
// instead of guessing an answer from memory.
const v1ListCapabilities = "Call this when the question asks what operations are " +
	"available - in general (\"何ができるの？\") or for one named service (\"在庫について、どういう" +
	"操作ができる？\") - rather than asking to actually look something up or change something. " +
	"Returns the catalogue's own list of operations, so it never risks naming a capability that " +
	"does not exist."

// v1ProposePanel is internal/usecase.proposePanelDescription as of
// 5bf5cf8, moved here unchanged: when to reach for propose_panel instead
// of calling a safe operation directly.
const v1ProposePanel = "Call this when the question asks to put something on the workspace's " +
	"screen - a panel, a chart, a table - rather than asking a question you should just answer. Name the " +
	"operation and arguments the panel's data should come from, exactly as you would for that operation's " +
	"own tool. component, chart, transform and title are all optional: leave any of them out and the " +
	"platform fills it in from the same rule it would have drawn the answer with. Never call this for a " +
	"question that only asks to look something up - call that operation's own tool instead."

// v1CatalogueTool renders a catalogue tool's description as exactly its
// endpoint's summary - today's behaviour
// (internal/usecase.ToolsFor/internal/adapter/planner/toolcall.shapeTool
// as of 5bf5cf8). examples is ignored: this is what keeps the wire
// byte-identical under v1 once domain.Endpoint.Examples /
// usecase.Tool.Examples exist (docs/plans/wording.md, Task 1, Step 3).
func v1CatalogueTool(summary string, _ []string) string {
	return summary
}

// v1 is the text in the product today, byte for byte (AC-Q-101;
// wording_test.go asserts every field against the literals copied
// straight from planner.go and tools.go at 5bf5cf8). It is a function,
// not a package-level value, for the same gochecknoglobals reason
// usecase.AskUserTool is a function (harness/quality/go/golangci.yml):
// callers each get their own Wording value rather than sharing - and
// risking a mutation of - one global.
func v1() Wording {
	return Wording{
		Name:             v1Name,
		SystemPrompt:     v1SystemPrompt,
		AskUser:          v1AskUser,
		ListCapabilities: v1ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
