package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

func TestHealthGetHealthReportsOK(t *testing.T) {
	h := handler.NewHealth()

	resp, err := h.GetHealth(t.Context(), openapi.GetHealthRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, openapi.GetHealth200JSONResponse(openapi.Health{Status: "ok"}), resp)
}
