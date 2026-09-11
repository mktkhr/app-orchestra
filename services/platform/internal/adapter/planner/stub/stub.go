// Package stub implements usecase.Planner as a table lookup: an exact
// query string maps to a fixed Decision, from a table its constructor
// takes. Nothing here guesses, calls a model, or performs any I/O, which
// is what lets it be the default planner in every test (the global
// constraint that `make check` never calls a real LLM) and, until Task 10
// adds a real adapter, the platform's only planner.
package stub

import (
	"context"
	"maps"

	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// Planner answers Plan by looking query up in a fixed table.
type Planner struct {
	table    map[string]usecase.Decision
	notFound usecase.Decision
}

var _ usecase.Planner = (*Planner)(nil)

// New builds a Planner over table: an exact query string maps to the
// Decision to return for it. A query outside the table returns *notFound
// (typically &usecase.Decision{Kind: usecase.DecisionNone}) rather than an
// error, so an un-fixtured question behaves the way a real planner finding
// nothing suitable would. A nil notFound is treated the same as
// &usecase.Decision{Kind: usecase.DecisionNone}.
//
// notFound is a pointer, not the value shown in the plan, because
// usecase.Decision is 112 bytes: golangci-lint's gocritic hugeParam check
// (part of the fixed harness policy, see harness/quality/go/golangci.yml)
// rejects passing it by value.
//
// table is copied, so a caller mutating the map they passed in cannot
// change this Planner's behaviour afterwards.
func New(table map[string]usecase.Decision, notFound *usecase.Decision) *Planner {
	copied := make(map[string]usecase.Decision, len(table))
	maps.Copy(copied, table)

	if notFound == nil {
		notFound = &usecase.Decision{Kind: usecase.DecisionNone}
	}

	return &Planner{table: copied, notFound: *notFound}
}

// Plan looks query up in the table. answers and tools are accepted only to
// satisfy usecase.Planner: the stub is a fixed mapping from query string to
// Decision, not a model that reads either.
func (p *Planner) Plan(_ context.Context, query string, _ []usecase.Answer, _ []usecase.Tool) (usecase.Decision, error) {
	if decision, ok := p.table[query]; ok {
		return decision, nil
	}

	return p.notFound, nil
}
