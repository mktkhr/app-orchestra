package wording

// v6Name is "v6-unmatched-filter".
const v6Name = "v6-unmatched-filter"

// v6SystemPromptAddition targets TODO.md item 3: a question whose
// restricting word matches no enum value gets every row back, silently -
// the model drops the filter and calls the operation with {}, answering
// "list everything" instead of the question actually asked
// (`no-enum-value`, 破損した在庫はある？; `no-enum-value-attendance`, 有給の
// 勤怠はある？). D15 (docs/specs/orchestration.md, a synthetic __all__ enum
// value) was tried and withdrawn (DECISIONS.md, 2026-09-12, final entry):
// it moved the accept rate, not the reject rate, because the synthetic
// value competed with ask_user as an easier tool to reach for rather than
// closing the silent-omission path. TODO.md item 3 says a fix has to
// change that competition through the system prompt, ask_user's own
// description, or the decision procedure - not add another enum value.
// This sentence is deliberately narrow: it fires only when the question
// names a restricting word (a state, kind or category) and the operation
// has an enum parameter for exactly that restriction, but the word
// matches none of the enum's values or labels. It does not fire for a
// question that does not restrict at all, and it does not touch
// free-text parameters - v3-ask-on-collision's mistake was a general
// "ask when unsure" rule that collapsed the corpus from 68 to 56
// correct@1 (scratchpad wording-report.txt; DECISIONS.md, 2026-09-15,
// "wording"); this rule is scoped to stay clear of that failure. It also
// states explicitly that dropping a restricting filter is not
// "committing" - v2-commit's own "call it when any offered tool plausibly
// fits" sentence must not be read as license to drop the one parameter
// the question was actually restricting on.
const v6SystemPromptAddition = " When the question restricts by a state, kind or category - a word " +
	"like 破損, 有給, 未処理 - and the operation you would call has a parameter with a fixed set of " +
	"allowed values for exactly that restriction, but the word matches none of those values or their " +
	"Japanese labels, do not call the operation with that parameter left out - that silently answers a " +
	"different question (every row, not the restricted ones). Call ask_user for that parameter instead, " +
	"so the person can pick from the set, or, only when one value clearly means the same thing the " +
	"question's word does, use that value. This does not apply when the question does not restrict at " +
	"all (call with no filter for a question like \"在庫を全部見せて\"), and it does not apply to a " +
	"free-text parameter that has no fixed set of values. Leaving a restricting filter out because its " +
	"word did not match is not \"committing\" - it is answering a different question than the one asked."

// v6AskUserAddition restates the same rule on ask_user's own description,
// in case the model reads a tool's own text before the system prompt's.
const v6AskUserAddition = " Also call this when the question's restricting word (a state, kind or " +
	"category) matches none of an enum parameter's values or their Japanese labels - dropping the " +
	"filter and calling the operation without it is not an option, since that would answer a different, " +
	"wider question than the one asked."

// v6SystemPrompt is v2SystemPrompt with v6SystemPromptAddition appended.
const v6SystemPrompt = v2SystemPrompt + v6SystemPromptAddition

// v6AskUser is v1AskUser with v6AskUserAddition appended.
const v6AskUser = v1AskUser + v6AskUserAddition

// v6UnmatchedFilter targets the open defect TODO.md item 3 describes: an
// unmatched restricting word against an enum parameter silently drops the
// filter instead of asking (see v6SystemPromptAddition's doc comment). It
// is built on v2-commit (SystemPrompt, ListCapabilities unchanged from
// v2Commit's own values) rather than v1, since v2-commit is
// wording.Default() and the base every further candidate extends.
// ProposePanel and CatalogueTool are v1's, unchanged.
func v6UnmatchedFilter() Wording {
	return Wording{
		Name:             v6Name,
		SystemPrompt:     v6SystemPrompt,
		AskUser:          v6AskUser,
		ListCapabilities: v2ListCapabilities,
		ProposePanel:     v1ProposePanel,
		CatalogueTool:    v1CatalogueTool,
	}
}
