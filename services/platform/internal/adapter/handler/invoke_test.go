package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakeInvoker is a test double for the invoker interface (what Invoke
// needs from the orchestration layer). It records the arguments it was
// called with and always returns the fixed result or error it was built
// with.
type fakeInvoker struct {
	result usecase.Result
	err    error

	user        *domain.User
	service     string
	operationID string
	args        map[string]any
}

func (f *fakeInvoker) Invoke(
	_ context.Context,
	user *domain.User,
	service, operationID string,
	args map[string]any,
) (usecase.Result, error) {
	f.user = user
	f.service = service
	f.operationID = operationID
	f.args = args

	return f.result, f.err
}

func TestPostInvokeRendersASuccessfulResult(t *testing.T) {
	orchestrator := &fakeInvoker{result: usecase.Result{
		Kind:      usecase.ResultKindResult,
		Component: domain.ComponentDetail,
		Data:      map[string]any{"id": "1"},
	}}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "CreateInventoryItem", Args: map[string]any{"name": "棚"}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke200JSONResponse)
	require.True(t, ok, "expected a 200 response, got %T", resp)
	assert.Equal(t, openapi.Component("detail"), body.Component)
	assert.Equal(t, map[string]any{"id": "1"}, body.Data)

	assert.Equal(t, "inventory", orchestrator.service)
	assert.Equal(t, "CreateInventoryItem", orchestrator.operationID)
	assert.Equal(t, map[string]any{"name": "棚"}, orchestrator.args)
}

func TestPostInvokeRendersAResultWithFields(t *testing.T) {
	orchestrator := &fakeInvoker{result: usecase.Result{
		Kind:      usecase.ResultKindResult,
		Component: domain.ComponentDetail,
		Data:      map[string]any{"status": "quarantined"},
		Fields: map[string]any{
			"status": map[string]any{
				"type":       "string",
				"enum":       []string{"quarantined"},
				"enumLabels": map[string]string{"quarantined": "検品保留"},
			},
		},
	}}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "CreateInventoryItem", Args: map[string]any{"name": "棚"}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke200JSONResponse)
	require.True(t, ok, "expected a 200 response, got %T", resp)

	require.NotNil(t, body.Fields)

	status, ok := (*body.Fields)["status"].(map[string]any)
	require.True(t, ok)

	enumLabels, ok := status["enumLabels"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "検品保留", enumLabels["quarantined"])
}

func TestPostInvokeRendersAResultWithNoFields(t *testing.T) {
	orchestrator := &fakeInvoker{result: usecase.Result{
		Kind:      usecase.ResultKindResult,
		Component: domain.ComponentDetail,
		Data:      map[string]any{"id": "1"},
	}}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "CreateInventoryItem", Args: map[string]any{"name": "棚"}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke200JSONResponse)
	require.True(t, ok, "expected a 200 response, got %T", resp)
	assert.Nil(t, body.Fields)
}

func TestPostInvokeMapsEndpointNotFoundTo400(t *testing.T) {
	orchestrator := &fakeInvoker{err: usecase.ErrEndpointNotFound}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "NoSuchOperation", Args: map[string]any{}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke400JSONResponse)
	require.True(t, ok, "expected a 400 response, got %T", resp)
	assert.NotEmpty(t, body.Message)
}

func TestPostInvokeMapsInvalidArgumentsTo400(t *testing.T) {
	orchestrator := &fakeInvoker{err: usecase.ErrInvalidArguments}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "CreateInventoryItem", Args: map[string]any{}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke400JSONResponse)
	require.True(t, ok, "expected a 400 response, got %T", resp)
	assert.NotEmpty(t, body.Message)
}

func TestPostInvokeMapsAnyOtherErrorTo500(t *testing.T) {
	orchestrator := &fakeInvoker{err: errors.New("service unreachable")}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "ListInventoryItems", Args: map[string]any{}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke500JSONResponse)
	require.True(t, ok, "expected a 500 response, got %T", resp)
	assert.NotEmpty(t, body.Message)
}

func TestPostInvokeMapsUnrenderableDataTo500(t *testing.T) {
	orchestrator := &fakeInvoker{result: usecase.Result{
		Kind: usecase.ResultKindResult,
		Data: []any{"not", "an", "object"},
	}}
	h := handler.NewInvoke(orchestrator)

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "ListInventoryItems", Args: map[string]any{}},
	})

	require.NoError(t, err)
	_, ok := resp.(openapi.PostInvoke500JSONResponse)
	require.True(t, ok, "expected a 500 response, got %T", resp)
}
