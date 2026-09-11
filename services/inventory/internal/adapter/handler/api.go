// Package handler implements the generated OpenAPI strict server interface,
// translating between the wire types oapi-codegen generated and the
// service's own domain types.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/inventory/internal/domain"
)

// itemStore is what Items needs from a repository. Satisfied by
// repository.Memory; a interface here keeps this package's dependency on the
// adapter layer's repository package one-directional and testable.
type itemStore interface {
	List(status *domain.Status) []domain.Item
	Get(id string) (domain.Item, bool)
	Create(n domain.NewItem) domain.Item
}

// Items implements the "items" and "system" tags of the generated strict
// server interface.
type Items struct {
	store itemStore
}

// NewItems builds the items handler over store.
func NewItems(store itemStore) *Items {
	return &Items{store: store}
}

// ListInventoryItems returns every item, optionally filtered by status.
func (h *Items) ListInventoryItems(
	_ context.Context,
	request openapi.ListInventoryItemsRequestObject,
) (openapi.ListInventoryItemsResponseObject, error) {
	var status *domain.Status
	if request.Params.Status != nil {
		s := domain.Status(*request.Params.Status)
		status = &s
	}

	items := h.store.List(status)

	out := make([]openapi.Item, 0, len(items))
	for _, item := range items {
		out = append(out, toAPIItem(item))
	}

	return openapi.ListInventoryItems200JSONResponse{Items: out}, nil
}

// GetInventoryItem returns a single item, or 404 when the id is unknown.
func (h *Items) GetInventoryItem(
	_ context.Context,
	request openapi.GetInventoryItemRequestObject,
) (openapi.GetInventoryItemResponseObject, error) {
	item, ok := h.store.Get(request.Id)
	if !ok {
		return openapi.GetInventoryItem404JSONResponse{
			Message: "no item exists with this id",
		}, nil
	}

	return openapi.GetInventoryItem200JSONResponse(toAPIItem(item)), nil
}

// CreateInventoryItem creates a new item and returns it.
func (h *Items) CreateInventoryItem(
	_ context.Context,
	request openapi.CreateInventoryItemRequestObject,
) (openapi.CreateInventoryItemResponseObject, error) {
	created := h.store.Create(domain.NewItem{
		Name:     request.Body.Name,
		Status:   domain.Status(request.Body.Status),
		Quantity: request.Body.Quantity,
	})

	return openapi.CreateInventoryItem201JSONResponse(toAPIItem(created)), nil
}

// GetInventorySpec serves the service's own contract, rendered as YAML from the spec
// oapi-codegen embedded in the generated code. Rendering it from there
// rather than from a second copy of api/openapi.yaml means the two can never
// drift: `make guard-generated` already guarantees the embedded spec matches
// the file on disk.
func (h *Items) GetInventorySpec(
	_ context.Context,
	_ openapi.GetInventorySpecRequestObject,
) (openapi.GetInventorySpecResponseObject, error) {
	specJSON, err := openapi.GetSpecJSON()
	if err != nil {
		return nil, fmt.Errorf("loading embedded spec: %w", err)
	}

	var doc any
	if unmarshalErr := json.Unmarshal(specJSON, &doc); unmarshalErr != nil {
		return nil, fmt.Errorf("parsing embedded spec: %w", unmarshalErr)
	}

	specYAML, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("rendering embedded spec as yaml: %w", err)
	}

	return openapi.GetInventorySpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(specYAML),
		ContentLength: int64(len(specYAML)),
	}, nil
}

// toAPIItem converts a domain.Item into its generated wire representation.
func toAPIItem(item domain.Item) openapi.Item {
	return openapi.Item{
		Id:       item.ID,
		Name:     item.Name,
		Status:   openapi.ItemStatus(item.Status),
		Quantity: item.Quantity,
	}
}
