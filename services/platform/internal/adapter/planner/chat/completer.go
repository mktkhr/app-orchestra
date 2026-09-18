package chat

import "context"

// Completer is what internal/adapter/planner/toolcall.Planner and
// internal/adapter/planner/pick.Picker actually depend on: one chat
// completion call, Request in, Response out. Client satisfies it, and so
// does internal/adapter/planner/chat/anthropic.Client - the second chat
// backend (ORCHESTRA_LLM_PROVIDER=anthropic) reuses this package's own
// Request/Response/Message/ToolCall shapes and FinishReasonLength constant
// rather than inventing parallel ones, so both planner stages work
// unchanged behind either transport.
type Completer interface {
	Complete(ctx context.Context, req *Request) (Response, error)
}

// Client itself satisfies Completer - asserted here so a signature change
// to either fails to compile at the point that breaks it, not wherever a
// caller first notices.
var _ Completer = (*Client)(nil)
