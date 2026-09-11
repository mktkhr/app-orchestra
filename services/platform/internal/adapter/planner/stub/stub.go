// Package stub implements usecase.Planner as a table lookup: an exact
// query string (plus, when present, the answers it was resubmitted with)
// maps to a fixed Decision, from a table its constructor takes. Nothing
// here guesses, calls a model, or performs any I/O, which is what lets it
// be the default planner in every test (the global constraint that `make
// check` never calls a real LLM) and, until Task 10 adds a real adapter,
// the platform's only planner.
package stub

import (
	"context"
	"maps"
	"sort"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// Key identifies one entry of the stub planner's table: a question, plus
// the canonical form of any answers it was resubmitted with (see
// AnswersKey). A question asked with no answers - the first ask, or a
// query the model would answer outright - uses the zero Answers value,
// "".
//
// A struct key, rather than a nested map, because Decision.Kind can differ
// for the very same query depending on whether it carries answers yet: an
// ambiguous question first comes back as an ask, and only produces a call
// once the person's answer is resubmitted alongside it
// (docs/plans/orchestration.md, Task 9, Step 2).
type Key struct {
	Query   string
	Answers string
}

// AnswersKey canonicalises answers into the string half of a Key: each
// answer as "param=value", sorted by param and joined with "&", so the
// same set of answers always produces the same key regardless of the order
// a caller built the slice in. No answers (nil or empty) canonicalises to
// "", the same key a bare question uses.
func AnswersKey(answers []usecase.Answer) string {
	if len(answers) == 0 {
		return ""
	}

	parts := make([]string, len(answers))
	for i, a := range answers {
		parts[i] = a.Param + "=" + a.Value
	}

	sort.Strings(parts)

	return strings.Join(parts, "&")
}

// Planner answers Plan by looking query and answers up in a fixed table.
type Planner struct {
	table    map[Key]usecase.Decision
	notFound usecase.Decision
}

var _ usecase.Planner = (*Planner)(nil)

// New builds a Planner over table: a Key maps to the Decision to return
// for it. A Key outside the table returns *notFound (typically
// &usecase.Decision{Kind: usecase.DecisionNone}) rather than an error, so
// an un-fixtured question behaves the way a real planner finding nothing
// suitable would. A nil notFound is treated the same as
// &usecase.Decision{Kind: usecase.DecisionNone}.
//
// notFound is a pointer, not the value shown in the plan, because
// usecase.Decision is 112 bytes: golangci-lint's gocritic hugeParam check
// (part of the fixed harness policy, see harness/quality/go/golangci.yml)
// rejects passing it by value.
//
// table is copied, so a caller mutating the map they passed in cannot
// change this Planner's behaviour afterwards.
func New(table map[Key]usecase.Decision, notFound *usecase.Decision) *Planner {
	copied := make(map[Key]usecase.Decision, len(table))
	maps.Copy(copied, table)

	if notFound == nil {
		notFound = &usecase.Decision{Kind: usecase.DecisionNone}
	}

	return &Planner{table: copied, notFound: *notFound}
}

// Plan looks {query, answers} up in the table. tools is accepted only to
// satisfy usecase.Planner: the stub is a fixed mapping from a query (and
// its answers, if any) to a Decision, not a model that reads it.
func (p *Planner) Plan(_ context.Context, query string, answers []usecase.Answer, _ []usecase.Tool) (usecase.Decision, error) {
	key := Key{Query: query, Answers: AnswersKey(answers)}
	if decision, ok := p.table[key]; ok {
		return decision, nil
	}

	return p.notFound, nil
}
