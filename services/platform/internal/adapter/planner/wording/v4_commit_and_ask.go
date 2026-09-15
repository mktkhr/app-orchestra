package wording

// v4Name is "v4-commit-and-ask".
const v4Name = "v4-commit-and-ask"

// v4SystemPrompt is v1SystemPrompt with both v2SystemPromptAddition and
// v3SystemPromptAddition appended, in that order.
const v4SystemPrompt = v1SystemPrompt + v2SystemPromptAddition + v3SystemPromptAddition

// v4CommitAndAsk is both deltas at once - v2-commit's on SystemPrompt and
// ListCapabilities, v3-ask-on-collision's on SystemPrompt and AskUser -
// targeting both misses their own doc comments name
// (v2SystemPromptAddition, v3SystemPromptAddition). ProposePanel and
// CatalogueTool are v1's, unchanged.
func v4CommitAndAsk() Wording {
	return Wording{
		Name:             v4Name,
		SystemPrompt:     v4SystemPrompt,
		AskUser:          v3AskUser,
		ListCapabilities: v2ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
