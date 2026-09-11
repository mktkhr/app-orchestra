package handler

import (
	"context"
	"errors"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// invoker is what Invoke needs from the orchestration layer: satisfied by
// *usecase.Orchestrator. An interface here, rather than the concrete type,
// keeps this handler's test doubles simple (see planner in plan.go).
type invoker interface {
	Invoke(ctx context.Context, service, operationID string, args map[string]any) (usecase.Result, error)
}

// Invoke implements the "invoke" tag of the generated strict server
// interface: POST /api/invoke. It executes a confirmed call - the
// endpoint lookup, argument validation and invocation itself all live in
// usecase.Orchestrator.Invoke; this handler only maps its outcome onto
// the wire.
type Invoke struct {
	orchestrator invoker
}

// NewInvoke builds the /api/invoke handler over orchestrator.
func NewInvoke(orchestrator invoker) *Invoke {
	return &Invoke{orchestrator: orchestrator}
}

// PostInvoke executes a confirmed call and renders its result.
func (h *Invoke) PostInvoke(
	ctx context.Context,
	request openapi.PostInvokeRequestObject,
) (openapi.PostInvokeResponseObject, error) {
	result, err := h.orchestrator.Invoke(ctx, request.Body.Service, request.Body.OperationId, request.Body.Args)
	if err != nil {
		return invokeErrorResponse(err), nil
	}

	data, err := toAPIData(result.Data)
	if err != nil {
		return invokeDataErrorResponse(err), nil
	}

	response := openapi.PostInvoke200JSONResponse{
		Component: openapi.Component(result.Component),
		Data:      data,
	}

	if len(result.Fields) > 0 {
		fields := result.Fields
		response.Fields = &fields
	}

	return response, nil
}

// invokeErrorResponse maps an Orchestrator.Invoke error onto an HTTP
// status: an unknown endpoint or arguments that fail validation are the
// caller's fault (400, docs/plans/orchestration.md Task 8); anything else
// - the service itself failing, most likely - is a 500.
func invokeErrorResponse(err error) openapi.PostInvokeResponseObject {
	if errors.Is(err, usecase.ErrEndpointNotFound) || errors.Is(err, usecase.ErrInvalidArguments) {
		return openapi.PostInvoke400JSONResponse{Message: err.Error()}
	}

	return openapi.PostInvoke500JSONResponse{Message: err.Error()}
}

// invokeDataErrorResponse reports a Result that could not be rendered onto
// the wire (see errUnrenderableData in plan.go) as a 500: this is a bug in
// the running deployment's endpoints, not something the caller did wrong.
func invokeDataErrorResponse(err error) openapi.PostInvokeResponseObject {
	return openapi.PostInvoke500JSONResponse{Message: err.Error()}
}
