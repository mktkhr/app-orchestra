package wording

// v7Name is "v7-verb".
const v7Name = "v7-verb"

// v7SystemPromptAddition targets the "wrong verb" shape in bonsai2-27b's
// axis-D misses (measurements, --corpus ext, 2026-09-19): a question that
// asks for something to be recorded, submitted or requested for the first
// time gets answered with an operation that reads or edits an existing
// record of the same resource instead of the one that creates it. The
// sentence names no resource, service or question - it states the general
// rule (match the verb, not just the resource) so it generalizes past the
// three misses that motivated it.
const v7SystemPromptAddition = " Match the operation to the verb the question asks for, not merely to " +
	"the resource: a question asking to record, submit or request something that does not already exist " +
	"calls the operation that creates it, never the one that reads or edits an existing record of that " +
	"same resource."

// v7SystemPrompt is v6SystemPrompt with v7SystemPromptAddition appended.
const v7SystemPrompt = v6SystemPrompt + v7SystemPromptAddition

// v7Verb targets the "wrong verb" shape in bonsai2-27b's axis-D misses: the
// right resource, the wrong operation on it (a read or an update where the
// question asked for a creation). It is built on v6-unmatched-filter -
// today's Default() - so it differs from it by exactly this one idea,
// measurable on its own. AskUser, ListCapabilities, ProposePanel and
// CatalogueTool are v6's, unchanged.
func v7Verb() Wording {
	return Wording{
		Name:             v7Name,
		SystemPrompt:     v7SystemPrompt,
		AskUser:          v6AskUser,
		ListCapabilities: v2ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
