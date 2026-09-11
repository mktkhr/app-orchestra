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

	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/domain"
)

// recordStore is what Records needs from a repository. Satisfied by
// repository.Memory; a interface here keeps this package's dependency on the
// adapter layer's repository package one-directional and testable.
type recordStore interface {
	List(kind *domain.Kind) []domain.Record
	Get(id string) (domain.Record, bool)
	Create(n domain.NewRecord) domain.Record
}

// Records implements the "records" and "system" tags of the generated strict
// server interface.
type Records struct {
	store recordStore
}

// NewRecords builds the records handler over store.
func NewRecords(store recordStore) *Records {
	return &Records{store: store}
}

// ListAttendanceRecords returns every record, optionally filtered by kind.
func (h *Records) ListAttendanceRecords(
	_ context.Context,
	request openapi.ListAttendanceRecordsRequestObject,
) (openapi.ListAttendanceRecordsResponseObject, error) {
	var kind *domain.Kind
	if request.Params.Kind != nil {
		k := domain.Kind(*request.Params.Kind)
		kind = &k
	}

	items := h.store.List(kind)

	out := make([]openapi.Record, 0, len(items))
	for _, item := range items {
		out = append(out, toAPIRecord(item))
	}

	return openapi.ListAttendanceRecords200JSONResponse{Records: out}, nil
}

// GetAttendanceRecord returns a single record, or 404 when the id is
// unknown.
func (h *Records) GetAttendanceRecord(
	_ context.Context,
	request openapi.GetAttendanceRecordRequestObject,
) (openapi.GetAttendanceRecordResponseObject, error) {
	item, ok := h.store.Get(request.Id)
	if !ok {
		return openapi.GetAttendanceRecord404JSONResponse{
			Message: "no record exists with this id",
		}, nil
	}

	return openapi.GetAttendanceRecord200JSONResponse(toAPIRecord(item)), nil
}

// CreateAttendanceRecord creates a new record and returns it.
func (h *Records) CreateAttendanceRecord(
	_ context.Context,
	request openapi.CreateAttendanceRecordRequestObject,
) (openapi.CreateAttendanceRecordResponseObject, error) {
	created := h.store.Create(domain.NewRecord{
		Employee: request.Body.Employee,
		Kind:     domain.Kind(request.Body.Kind),
		Date:     request.Body.Date,
	})

	return openapi.CreateAttendanceRecord201JSONResponse(toAPIRecord(created)), nil
}

// GetSpec serves the service's own contract, rendered as YAML from the spec
// oapi-codegen embedded in the generated code. Rendering it from there
// rather than from a second copy of api/openapi.yaml means the two can never
// drift: `make guard-generated` already guarantees the embedded spec matches
// the file on disk.
func (h *Records) GetSpec(
	_ context.Context,
	_ openapi.GetSpecRequestObject,
) (openapi.GetSpecResponseObject, error) {
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

	return openapi.GetSpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(specYAML),
		ContentLength: int64(len(specYAML)),
	}, nil
}

// toAPIRecord converts a domain.Record into its generated wire
// representation.
func toAPIRecord(item domain.Record) openapi.Record {
	return openapi.Record{
		Id:       item.ID,
		Employee: item.Employee,
		Kind:     openapi.RecordKind(item.Kind),
		Date:     item.Date,
	}
}
