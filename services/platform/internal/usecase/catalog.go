package usecase

import (
	"context"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// CatalogEntry is one operation a person may build a panel over
// (docs/specs/dashboard.md, section 5): what GET /api/catalog lists, one
// per endpoint of Catalog.For(ctx, user).
type CatalogEntry struct {
	Service     string
	OperationID string
	Summary     string
	Component   domain.Component
	// Schema is the call's arguments, from inputSchemaFor - the same
	// shape a kind: form Result carries as its own Schema, so the browser
	// builds a panel's argument form the same way it builds a
	// confirmation form (docs/specs/dashboard.md, P7).
	Schema map[string]any
	// Fields describes the response's fields, from domain.FieldsSchema,
	// converted the same way fieldsFor converts a Result's own Fields -
	// nil when domain.FieldsSchema finds nothing to describe, never an
	// empty map (see fieldsFor's own doc comment).
	Fields map[string]any
	// View carries the endpoint's contract-declared chart axes, exactly
	// as chartViewFor builds it for a Result (docs/specs/dashboard.md,
	// P2) - nil when the contract declares none.
	View *domain.View
}

// Catalog is the usecase behind GET /api/catalog: every operation the
// signed-in person may call, each described well enough to build a panel
// from (docs/specs/dashboard.md, section 5).
type Catalog struct {
	catalog     domain.Catalog
	permissions PermissionStore
}

// NewCatalog builds a Catalog usecase over the platform's whole catalogue,
// narrowed per request by permissions.
func NewCatalog(catalog domain.Catalog, permissions PermissionStore) *Catalog {
	return &Catalog{catalog: catalog, permissions: permissions}
}

// For lists every operation user may call, narrowed by the shared
// catalogFor (auth.go) - the same function Orchestrator and Workspaces
// narrow through, so the operations a person can build a panel from are
// the same set they can ask a question about or save a panel over
// (docs/specs/auth.md, section 5; docs/plans/dashboard.md, Task 4).
//
// This is deliberately not GET /api/operations' Admin.Operations: that
// method answers what the deployment holds, unfiltered, for the admin's
// permission grid; this answers what the calling person may call, and
// carries the arguments schema and response fields a panel builder needs
// besides (docs/specs/dashboard.md, section 5).
func (c *Catalog) For(ctx context.Context, user *domain.User) ([]CatalogEntry, error) {
	catalog, err := catalogFor(ctx, c.catalog, c.permissions, user)
	if err != nil {
		return nil, fmt.Errorf("narrowing the catalogue: %w", err)
	}

	entries := make([]CatalogEntry, len(catalog.Endpoints))
	for i := range catalog.Endpoints {
		entries[i] = toCatalogEntry(&catalog.Endpoints[i])
	}

	return entries, nil
}

// toCatalogEntry builds one CatalogEntry from an endpoint, reusing exactly
// the two conversions a /api/plan result already goes through for the
// same two jobs: inputSchemaFor for the arguments (see formFor) and
// fieldsFor for the response's fields (see invokeAndRender) - a
// catalogue entry describes the same two shapes a plan result does,
// because the browser builds the same two things from them (a form, and a
// chart's or transform's field list).
func toCatalogEntry(e *domain.Endpoint) CatalogEntry {
	return CatalogEntry{
		Service:     e.Service,
		OperationID: e.OperationID,
		Summary:     e.Summary,
		Component:   domain.Render(e),
		Schema:      inputSchemaFor(e),
		Fields:      fieldsFor(e),
		View:        chartViewFor(e),
	}
}
