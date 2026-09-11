// Package handler implements the generated OpenAPI server interface.
package handler

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

// Health implements the "system" tag of the generated strict server
// interface: today, just GET /api/health.
type Health struct{}

// NewHealth builds the health handler.
func NewHealth() *Health {
	return &Health{}
}

// GetHealth reports that the process is up.
func (h *Health) GetHealth(
	_ context.Context,
	_ openapi.GetHealthRequestObject,
) (openapi.GetHealthResponseObject, error) {
	return openapi.GetHealth200JSONResponse(openapi.Health{Status: "ok"}), nil
}
