package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

func TestPostInvokeIsNotImplemented(t *testing.T) {
	h := handler.NewInvoke()

	resp, err := h.PostInvoke(t.Context(), openapi.PostInvokeRequestObject{
		Body: &openapi.InvokeRequest{Service: "inventory", OperationId: "CreateInventoryItem", Args: map[string]any{}},
	})

	require.NoError(t, err)
	body, ok := resp.(openapi.PostInvoke501JSONResponse)
	require.True(t, ok, "expected a 501 response, got %T", resp)
	assert.NotEmpty(t, body.Message)
}
