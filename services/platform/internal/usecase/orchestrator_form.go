package usecase

import (
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// Split from orchestrator.go (harness/quality/file-length.txt's 1000-line
// guard): formFor and the drop-invented-initials rule it applies
// (2026-09-16, TODO.md "invented form values"), plus every helper it needs.

// formFor builds the form the platform hands a person instead of running
// something: the endpoint's whole argument schema, and whatever arguments
// the model did manage to fill in as its initial values - minus the ones
// dropInventedInitials decides the model invented rather than read off the
// question (see its own doc comment).
//
// All three places that produce one are the same idea, which is why they
// share this function rather than each writing the literal out. Two of them
// are the same rule twice over - an unsafe operation is answered by its
// form, whether the model tried to call it (call) or reached for ask_user
// on an argument it could not fill (ask), because D8 says the model never
// runs an unsafe operation at all and a person pressing the button is what
// does (docs/specs/orchestration.md, section 8b). The third is D11's own
// degradation: a safe operation whose stuck argument has no enum has no
// list of values to offer, and letting the person type it is what a form
// is for.
//
// query and answers are the same question (and any values already
// confirmed for it) every caller already has in hand - threaded through
// here only so dropInventedInitials has something to check an invented
// value against.
func formFor(endpoint *domain.Endpoint, decision *Decision, query string, answers []Answer) Result {
	schema := inputSchemaFor(endpoint)

	return Result{
		Kind:               ResultKindForm,
		Service:            decision.Service,
		ServiceDisplayName: endpoint.ServiceDisplayNameOr(decision.Service),
		OperationID:        decision.OperationID,
		Schema:             schema,
		Initial:            dropInventedInitials(schema, decision.Args, query, answers),
	}
}

// dropInventedInitials removes a form's initial value for any parameter the
// model most likely invented rather than read off the question - fixed
// 2026-09-16 (TODO.md "invented form values", DECISIONS.md "Thirty
// questions against the real dev services"): 「遅刻を記録したい」 filled
// employee with 田中太郎 and 「新しい在庫を登録したい」 filled name with
// 「新しい在庫」, neither ever said by the person asking. A form is the
// person's to complete, and a name they never said is not a default, it is
// a fabrication that reads as data - so it is dropped, left for them to
// type, rather than shown as if it meant something.
//
// The rule touches exactly one shape of parameter: schema type "string"
// with no enum and no format. An enum is never free text - the model
// picked from a declared list, the same trust ask's own options already
// place in it. A format (date, date-time, or any other the catalogue
// declares) names a value the platform itself can compute a sensible
// default for - today's date, most often, now that the planner is told
// what day it is (toolcall.WithClock/jsonmode.WithClock) - not something
// only the person asking could have said, so it is kept even when it does
// not appear in the question. A number or a boolean is never distinguished
// from a real value by this rule either - `quantity: 1` reads exactly as
// plausible typed as guessed, and there is no text to compare it against.
//
// What survives the type/enum/format filter is kept only if it - trimmed -
// appears in query (the question itself) or in one of answers (a value
// already confirmed for a previous ask_user question on the same
// conversation), by ordinary containment or an ASCII case-insensitive one;
// otherwise it is dropped. args itself (nil or empty) passes through
// unchanged: there is nothing to filter, and the "nil, not an empty map,
// for nothing here" convention argsFromAnswers already uses elsewhere only
// matters once filtering has actually removed something.
func dropInventedInitials(schema, args map[string]any, query string, answers []Answer) map[string]any {
	if len(args) == 0 {
		return args
	}

	properties, ok := schema[keyProperties].(map[string]any)
	if !ok {
		properties = nil
	}

	kept := make(map[string]any, len(args))

	for name, value := range args {
		if keepInitialValue(properties, name, value, query, answers) {
			kept[name] = value
		}
	}

	if len(kept) == 0 {
		return nil
	}

	return kept
}

// keepInitialValue decides one parameter's fate for dropInventedInitials:
// true for anything outside the rule's scope (no schema found for name, not
// a plain string, an enum, or a declared format) or a plain string value
// that mentionedInQuestion finds attested; false - drop it - only for a
// plain free-text string the question and its answers never mention.
func keepInitialValue(properties map[string]any, name string, value any, query string, answers []Answer) bool {
	prop, ok := properties[name].(map[string]any)
	if !ok {
		return true
	}

	if prop[keyType] != domain.SchemaTypeString {
		return true
	}

	if _, hasEnum := prop[keyEnum]; hasEnum {
		return true
	}

	if format, hasFormat := prop[keyFormat].(string); hasFormat && format != "" {
		return true
	}

	s, ok := value.(string)
	if !ok {
		return true
	}

	return mentionedInQuestion(s, query, answers)
}

// mentionedInQuestion reports whether value - trimmed - appears in query or
// in one of answers' own Value, either by plain strings.Contains or by an
// ASCII case-insensitive one (strings.ToLower is a no-op on Japanese text,
// so this never changes how a Japanese value matches - it only helps an
// ASCII name typed or answered in different case). An empty (after
// trimming) value is never considered mentioned - there is nothing there to
// have said.
func mentionedInQuestion(value, query string, answers []Answer) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	if mentioned(trimmed, query) {
		return true
	}

	for _, a := range answers {
		if mentioned(trimmed, a.Value) {
			return true
		}
	}

	return false
}

// mentioned reports whether value appears in text, by plain
// strings.Contains or by an ASCII case-insensitive one - see
// mentionedInQuestion's own doc comment for why both.
func mentioned(value, text string) bool {
	if strings.Contains(text, value) {
		return true
	}

	return strings.Contains(strings.ToLower(text), strings.ToLower(value))
}
