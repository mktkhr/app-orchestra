package wording

// v2Name is "v2-commit".
const v2Name = "v2-commit"

// v2SystemPromptAddition targets the two largest single miss groups in
// DECISIONS.md, 2026-09-15 "The product's planner, measured end to end":
// 10 of 35 misses are "none" where the picker committed to a real
// operation, and list_capabilities was returned as the answer to a vague,
// answerable question (docs/specs/wording.md, section 1, 4: five times in
// 100). The sentence tells the model to commit when any offered tool
// plausibly fits, rather than retreating to list_capabilities or nothing
// out of caution.
const v2SystemPromptAddition = " When any of the offered tools plausibly serves the question, call it - " +
	"do not answer with list_capabilities, or with no tool at all, merely because you are unsure which " +
	"one fits best. list_capabilities is for a person asking what the system can do, in general or for " +
	"one named service, never for a question one of the offered tools could already answer."

// v2ListCapabilitiesAddition restates the same rule on list_capabilities'
// own description, in case the model reads a tool's own text before the
// system prompt's.
const v2ListCapabilitiesAddition = " Never call this for a question that one of the offered tools could " +
	"answer directly - list_capabilities is for \"what can this do\", not for \"do this\"."

// v2SystemPrompt is v1SystemPrompt with v2SystemPromptAddition appended.
const v2SystemPrompt = v1SystemPrompt + v2SystemPromptAddition

// v2ListCapabilities is v1ListCapabilities with v2ListCapabilitiesAddition
// appended.
const v2ListCapabilities = v1ListCapabilities + v2ListCapabilitiesAddition

// v2Commit targets `none` where the picker commits and list_capabilities
// returned as the answer (see v2SystemPromptAddition's doc comment).
// AskUser, ProposePanel and CatalogueTool are v1's, unchanged.
func v2Commit() Wording {
	return Wording{
		Name:             v2Name,
		SystemPrompt:     v2SystemPrompt,
		AskUser:          v1AskUser,
		ListCapabilities: v2ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
