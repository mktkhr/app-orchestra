package handler

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

// notImplementedInvokeMessage explains, to a caller, why POST /api/invoke
// answers 501: the contract exists (docs/plans/orchestration.md, Task 6
// adds it alongside /api/plan) but the execution path itself is Task 8's
// job.
const notImplementedInvokeMessage = "POST /api/invoke is not implemented yet (docs/plans/orchestration.md, Task 8)."

// Invoke implements the "invoke" tag of the generated strict server
// interface: POST /api/invoke. It is a placeholder that always reports 501
// so the generated interface is fully implemented (and the frontend can
// already be built against the contract) while Task 8 builds the real
// execution path: looking the endpoint up in the catalogue, validating its
// arguments, invoking it and rendering the result.
type Invoke struct{}

// NewInvoke builds the placeholder /api/invoke handler.
func NewInvoke() *Invoke {
	return &Invoke{}
}

// PostInvoke always answers 501, until Task 8 implements execution.
func (h *Invoke) PostInvoke(
	_ context.Context,
	_ openapi.PostInvokeRequestObject,
) (openapi.PostInvokeResponseObject, error) {
	return openapi.PostInvoke501JSONResponse{Message: notImplementedInvokeMessage}, nil
}
