package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// DecisionKind names what a Planner decided to do about a question: call
// one endpoint, ask the user to disambiguate a value, or report that
// nothing in the catalogue fits.
type DecisionKind string

// The three things a Planner can decide (docs/specs/orchestration.md,
// section 4). Only DecisionCall against a safe endpoint, and DecisionNone,
// are acted on by Orchestrator today: an unsafe DecisionCall (the form
// path, Task 7) and DecisionAsk (Task 9) return ErrNotImplemented.
const (
	DecisionCall DecisionKind = "call"
	DecisionAsk  DecisionKind = "ask"
	DecisionNone DecisionKind = "none"
)

// Answer is the user's answer to a previous ask_user question, resubmitted
// alongside the original query so the planner can try again with it.
type Answer struct {
	Param string
	Value string
}

// Decision is what a Planner returns: which endpoint to call and with what
// arguments, or a question to ask, or nothing.
type Decision struct {
	Kind DecisionKind

	// Populated when Kind is DecisionCall.
	Service     string
	OperationID string
	Args        map[string]any

	// Populated when Kind is DecisionAsk.
	Question string
	Param    string
	Options  []domain.Option
}

// Planner decides, from a question and the tools describing the
// catalogue, what to do about it. Implemented by internal/adapter/planner/
// stub (a deterministic table lookup, used by every test and, by default,
// by the running platform - see pkg/app) and, from Task 10, by an adapter
// that calls a real LLM.
type Planner interface {
	Plan(ctx context.Context, query string, answers []Answer, tools []Tool) (Decision, error)
}

// Invoker calls one endpoint of one service and returns its decoded JSON
// result. Implemented by internal/adapter/invoker/http.
//
// e is a pointer, not the value shown in the plan, because domain.Endpoint
// is 136 bytes: golangci-lint's gocritic hugeParam check (part of the fixed
// harness policy, see harness/quality/go/golangci.yml) rejects passing it
// by value - the same reason domain.Render and Endpoint.IsSafe take one.
type Invoker interface {
	Invoke(ctx context.Context, e *domain.Endpoint, args map[string]any) (any, error)
}
