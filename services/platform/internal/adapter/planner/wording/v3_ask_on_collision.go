package wording

// v3Name is "v3-ask-on-collision".
const v3Name = "v3-ask-on-collision"

// v3SystemPromptAddition and v3AskUserAddition target the under-asking
// DECISIONS.md, 2026-09-15 "The product's planner, measured end to end"
// found: the planner returned ask_user exactly once in 100 answers
// (docs/specs/wording.md, section 1, 4: one ask in 100), against the
// stand-in picker flagging axis B (the ambiguous axis) 52% of the time on
// the identical shortlists - roughly half of the questions built to be
// genuinely ambiguous. The corpus's own ambiguous pairs are named
// directly: 受注 (sales orders) and 発注 (purchasing orders) are both 注文
// ("an order"), and 勤怠 (attendance) and 経費 (expense) both have 社員
// ("an employee"). The sentences tell the model this specific collision -
// two offered tools that differ only in which service owns the same kind
// of operation - is exactly the case it must not guess through.
const v3SystemPromptAddition = " When two or more of the offered tools differ only in which service " +
	"owns the same kind of operation - 受注 and 発注 are both 注文, 勤怠 and 経費 both have 社員 - do " +
	"not guess which one was meant: call ask_user, naming both operations in the question, and let the " +
	"person choose."

// v3AskUserAddition restates the same rule on ask_user's own description,
// in case the model reads a tool's own text before the system prompt's,
// and tells it what to do with operationId - a single-value argument -
// when two operations collide: name both in the question text, and set
// operationId to whichever one it would otherwise have called.
const v3AskUserAddition = " Also call this - never guess - when two or more of the offered tools " +
	"differ only in which service owns the same kind of operation (受注 and 発注 are both 注文; 勤怠 and " +
	"経費 both have 社員): name both operations in the question, and set operationId to whichever one " +
	"you would otherwise have called."

// v3SystemPrompt is v1SystemPrompt with v3SystemPromptAddition appended.
const v3SystemPrompt = v1SystemPrompt + v3SystemPromptAddition

// v3AskUser is v1AskUser with v3AskUserAddition appended.
const v3AskUser = v1AskUser + v3AskUserAddition

// v3AskOnCollision targets the one ask in 100 (see
// v3SystemPromptAddition's doc comment). ListCapabilities, ProposePanel
// and CatalogueTool are v1's, unchanged.
func v3AskOnCollision() Wording {
	return Wording{
		Name:             v3Name,
		SystemPrompt:     v3SystemPrompt,
		AskUser:          v3AskUser,
		ListCapabilities: v1ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
