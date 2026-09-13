package handler

import (
	"context"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// catalog is what Catalog needs from the usecase layer: satisfied by
// *usecase.Catalog. An interface here, rather than the concrete type,
// keeps this handler's test doubles simple (see planner in plan.go).
type catalog interface {
	For(ctx context.Context, user *domain.User) ([]usecase.CatalogEntry, error)
}

// Catalog implements the "catalog" tag of the generated strict server
// interface: GET /api/catalog.
type Catalog struct {
	catalog catalog
}

// NewCatalog builds the /api/catalog handler over c.
func NewCatalog(c catalog) *Catalog {
	return &Catalog{catalog: c}
}

// GetCatalog implements GET /api/catalog: every operation the signed-in
// person may call, each described well enough to build a panel from
// (docs/specs/dashboard.md, section 5).
func (h *Catalog) GetCatalog(
	ctx context.Context,
	_ openapi.GetCatalogRequestObject,
) (openapi.GetCatalogResponseObject, error) {
	entries, err := h.catalog.For(ctx, currentUser(ctx))
	if err != nil {
		return nil, fmt.Errorf("reading the catalogue: %w", err)
	}

	out := make(openapi.GetCatalog200JSONResponse, len(entries))
	for i := range entries {
		out[i] = toAPICatalogEntry(&entries[i])
	}

	return out, nil
}

// toAPICatalogEntry converts one usecase.CatalogEntry into the wire
// CatalogEntry.
func toAPICatalogEntry(e *usecase.CatalogEntry) openapi.CatalogEntry {
	out := openapi.CatalogEntry{
		Service:            e.Service,
		ServiceDisplayName: e.ServiceDisplayName,
		OperationId:        e.OperationID,
		Summary:            e.Summary,
		DisplayName:        e.DisplayName,
		Component:          openapi.Component(e.Component),
		Schema:             e.Schema,
	}

	if len(e.Fields) > 0 {
		fields := e.Fields
		out.Fields = &fields
	}

	out.View = toAPIView(e.View)

	return out
}
