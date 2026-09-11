package handler

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// currentUser resolves the person a request is running as, and is the one
// call site every handler in this package goes through to get one
// (Plan.PostPlan, Invoke.PostInvoke, Workspace's own methods) - so wiring
// docs/plans/auth.md Task 2's session middleware in later is a change to
// this function's body alone, the same way pkg/app's own seedAdmin doc
// comment describes for its own seat.
//
// Task 2 has not landed yet: nothing between the browser and this
// function resolves a session cookie into a user, so every request today
// runs as a fixed admin. That stub lives nowhere but here - every usecase
// this package calls (Orchestrator.Plan, Orchestrator.Invoke, every
// Workspaces method) takes the user as an argument
// (docs/specs/auth.md, section 5, A6), so once Task 2's middleware starts
// setting a real user on ctx, changing this one function to read it back
// out is the whole of the change every call site needs.
func currentUser(_ context.Context) *domain.User {
	return &domain.User{ID: "stub-admin", Role: domain.RoleAdmin}
}
