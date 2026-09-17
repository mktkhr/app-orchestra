package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// Split from orchestrator.go: the enum-guess guard that restores AC-B-105
// (PRODUCT.md) under two-stage planning (default since 9802e91). Under the
// single call the fill either read a status off the question or it did
// not; under planStaged's separate fill call it sometimes writes a
// catalogue-valid enum value the question never named at all - measured
// 2026-09-17: 「破損した在庫はある？」 -> ListInventoryItems{status:
// quarantined}, 「有給の勤怠はある？」 -> ListAttendanceRecords{kind:
// compensatory} (有給 is paid leave; 代休 is a different thing entirely).
// call is where both planOrdinary's and planPreferred's DecisionCall
// reach the catalogue, so this is the one place the guard needs to sit.

// minSharedEnumLabelRun is the shortest run of consecutive runes between a
// guessed value's Japanese label and the question (or an answer) that
// counts as the label having actually been named, rather than merely
// resembled: one rune alone is too weak a signal in Japanese, where a
// single kanji is often common to several unrelated labels. Two is enough
// to tell 「検品中のやつ」 (shares 検品 with 検品保留) from 「破損した」
// (shares nothing with any inventory status label).
const minSharedEnumLabelRun = 2

// enumGuess is one argument call's guard decided the fill invented: an
// enum-valued parameter whose value is neither itself, nor its label,
// mentioned anywhere in the question or the answers already given.
type enumGuess struct {
	Param string
	Value string
}

// guessedEnumArgs reports every argument of decision.Args that names one
// of endpoint's enum-typed parameters (endpoint.Parameters - the same
// scope optionsForParam already searches, orchestrator_ask.go) with a
// value enumValueIsGuess judges invented.
//
// Scoped to Parameters, deliberately not endpoint.RequestBody: a request
// body only ever appears on an unsafe operation's create (the same reason
// optionsForParam's own doc comment gives for not searching one), and
// dropInventedInitials (orchestrator_form.go) already has its own,
// broader rule for that shape - "an enum is never free text ... the same
// trust ask's own options already place in it" - trusting every model-filled
// enum on a create's form outright. Extending this guard to a create's
// request body would take that trust back for exactly the values this
// guard calls guessed, which is a real behaviour change orchestrator_form_initials_test.go's
// existing cases were never written to expect. The two real defects this
// guard restores AC-B-105 for - ListInventoryItems' status,
// ListAttendanceRecords' kind - are both query parameters on safe list
// operations, not request-body properties on a create, so Parameters
// alone is enough to fix them.
func guessedEnumArgs(endpoint *domain.Endpoint, args map[string]any, query string, answers []Answer) []enumGuess {
	if len(args) == 0 {
		return nil
	}

	var guesses []enumGuess

	for i := range endpoint.Parameters {
		p := &endpoint.Parameters[i]
		if g, ok := enumGuessFor(&p.Schema, p.Name, args, query, answers); ok {
			guesses = append(guesses, g)
		}
	}

	return guesses
}

// enumGuessFor checks one named argument against one schema: it reports
// false for anything outside the guard's scope at all (no enum declared,
// no value given for name, or a value that is not even one of the
// declared enum values - validateArgs' own job, not this guard's), and
// otherwise defers to enumValueIsGuess.
func enumGuessFor(schema *domain.Schema, name string, args map[string]any, query string, answers []Answer) (enumGuess, bool) {
	if len(schema.Enum) == 0 {
		return enumGuess{}, false
	}

	raw, ok := args[name]
	if !ok {
		return enumGuess{}, false
	}

	value := fmt.Sprint(raw)
	if !slices.Contains(schema.Enum, value) {
		return enumGuess{}, false
	}

	if !enumValueIsGuess(schema, value, query, answers) {
		return enumGuess{}, false
	}

	return enumGuess{Param: name, Value: value}, true
}

// enumValueIsGuess reports whether value - already known to be one of
// schema's declared Enum values - is something the question or an answer
// actually named, or something the fill invented: a guess is anything
// that is neither the value itself, nor its EnumLabels label, mentioned
// by mentionedInQuestion (a2c7503's own drop rule, reused rather than
// duplicated here), nor sharing a run of minSharedEnumLabelRun or more
// consecutive runes between the label and the question or an answer.
//
// A value with no EnumLabels entry - not something a spec-conformant
// service can produce, but not ruled out by the type system either, the
// same defensive case optionsFromSchema already handles - has nothing
// left to check beyond mentionedInQuestion's own two comparisons, and
// mentionedInQuestion already ran on the bare value above, so it counts
// as a guess outright.
func enumValueIsGuess(schema *domain.Schema, value, query string, answers []Answer) bool {
	if mentionedInQuestion(value, query, answers) {
		return false
	}

	label := schema.EnumLabels[value]
	if label == "" {
		return true
	}

	if mentionedInQuestion(label, query, answers) {
		return false
	}

	if sharesRun(label, query, minSharedEnumLabelRun) {
		return false
	}

	for _, a := range answers {
		if sharesRun(label, a.Value, minSharedEnumLabelRun) {
			return false
		}
	}

	return true
}

// sharesRun reports whether label and text share a contiguous run of at
// least minRunes runes: some substring of label, exactly minRunes runes
// long, appears in text as a substring. Checking every minRunes-long
// window of label is enough to find any longer shared run too - a run of
// length n >= minRunes contains n-minRunes+1 windows of length minRunes,
// each one itself a substring of the longer run, so if the longer run is
// shared, at least one of its minRunes windows is shared on its own.
//
// Runes, not bytes: minRunes counts Japanese characters, each three bytes
// long in UTF-8, and slicing by byte index would as likely as not cut one
// in half.
func sharesRun(label, text string, minRunes int) bool {
	runes := []rune(label)

	for i := 0; i+minRunes <= len(runes); i++ {
		if strings.Contains(text, string(runes[i:i+minRunes])) {
			return true
		}
	}

	return false
}

// enumParamTitle answers the display name askForEnumGuess's question
// names a guessed parameter by: its own schema.Title ("ステータス",
// "種別"), the same title a contract already declares for its select in a
// form, falling back to the bare parameter name when the contract
// declares none. Only Parameters is searched, the same scope
// guessedEnumArgs itself found the guess in.
func enumParamTitle(endpoint *domain.Endpoint, name string) string {
	for i := range endpoint.Parameters {
		if endpoint.Parameters[i].Name != name {
			continue
		}

		if title := endpoint.Parameters[i].Schema.Title; title != "" {
			return title
		}

		break
	}

	return name
}

// askForEnumGuess turns one safe call's guessed argument into a
// ResultKindAsk over that same parameter, offering the catalogue's own
// options - never decision.Args' guessed value, and never a value the
// model invented - exactly as an ordinary DecisionAsk over an enum
// already does (ask, orchestrator_ask.go): the guard's whole point is
// that a guess deserves the same question a model-issued ask_user would
// have produced, not a different shape of answer.
func askForEnumGuess(ctx context.Context, endpoint *domain.Endpoint, g enumGuess) Result {
	logEnumGuessReplaced(ctx, g)

	options, _ := optionsForParam(endpoint, g.Param)

	return Result{
		Kind:     ResultKindAsk,
		Question: enumParamTitle(endpoint, g.Param) + "はどれですか？",
		Param:    g.Param,
		Options:  options,
	}
}

// dropEnumGuesses copies decision with every guessed argument removed
// from its Args, for an unsafe operation: D8 already answers an unsafe
// call with a form to confirm (formFor), and the guard's job there is
// only to keep the guessed value from ever reaching that form as if the
// person had said it - the empty enum select this leaves behind is
// exactly what a person choosing for themselves looks like. decision is
// copied, not mutated in place, because it is the caller's own value
// (Plan holds the Decision the planner returned, unrelated calls of call
// share it via planPreferred's retry).
func dropEnumGuesses(ctx context.Context, decision *Decision, guesses []enumGuess) *Decision {
	kept := make(map[string]any, len(decision.Args))
	maps.Copy(kept, decision.Args)

	for _, g := range guesses {
		delete(kept, g.Param)
		logEnumGuessReplaced(ctx, g)
	}

	if len(kept) == 0 {
		kept = nil
	}

	d := *decision
	d.Args = kept

	return &d
}

// logEnumGuessReplaced is the one info-level log line the guard emits,
// once per guessed argument, whichever way it was resolved (turned into
// an ask, or dropped from an unsafe call's form): param and value, so an
// operator can see what the fill invented without reading a full request
// trace.
func logEnumGuessReplaced(ctx context.Context, g enumGuess) {
	slog.Default().InfoContext(ctx, "enum_guess_replaced",
		slog.String("param", g.Param), slog.String("value", g.Value))
}
