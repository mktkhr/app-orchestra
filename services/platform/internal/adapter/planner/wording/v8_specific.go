package wording

// v8Name is "v8-specific".
const v8Name = "v8-specific"

// v8SystemPromptAddition targets the "neighbouring or more general
// resource" shape in bonsai2-27b's axis-D misses (measurements, --corpus
// ext, 2026-09-19): the offered tools include both a specific operation and
// a more general one that also technically fits, and the picker reaches
// for the general one instead of the specific one the question actually
// names. The sentence names no resource, service or question - it states
// the general rule (prefer the named specific over the also-fitting
// general) so it generalizes past the eight misses that motivated it.
const v8SystemPromptAddition = " When the candidate operations include both a general resource and a " +
	"more specific one that would also technically fit, call the one whose name matches the specific " +
	"thing the question actually names, not the broader one it happens to also satisfy."

// v8SystemPrompt is v6SystemPrompt with v8SystemPromptAddition appended.
const v8SystemPrompt = v6SystemPrompt + v8SystemPromptAddition

// v8Specific targets the "neighbouring or more general resource" shape in
// bonsai2-27b's axis-D misses: the right family, the wrong member, because
// a more general operation also technically fits. It is built on
// v6-unmatched-filter - today's Default() - so it differs from it by
// exactly this one idea, measurable on its own. AskUser, ListCapabilities,
// ProposePanel and CatalogueTool are v6's, unchanged.
func v8Specific() Wording {
	return Wording{
		Name:             v8Name,
		SystemPrompt:     v8SystemPrompt,
		AskUser:          v6AskUser,
		ListCapabilities: v2ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
