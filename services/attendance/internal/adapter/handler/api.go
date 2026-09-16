// Package handler implements the generated OpenAPI strict server interface,
// translating between the wire types oapi-codegen generated and the
// service's own domain types.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
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
		record, err := toAPIRecord(item)
		if err != nil {
			return nil, err
		}

		out = append(out, record)
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

	record, err := toAPIRecord(item)
	if err != nil {
		return nil, err
	}

	return openapi.GetAttendanceRecord200JSONResponse(record), nil
}

// CreateAttendanceRecord creates a new record and returns it.
func (h *Records) CreateAttendanceRecord(
	_ context.Context,
	request openapi.CreateAttendanceRecordRequestObject,
) (openapi.CreateAttendanceRecordResponseObject, error) {
	created := h.store.Create(domain.NewRecord{
		Employee: request.Body.Employee,
		Kind:     domain.Kind(request.Body.Kind),
		Date:     request.Body.Date.Format(openapi_types.DateFormat),
	})

	record, err := toAPIRecord(created)
	if err != nil {
		return nil, err
	}

	return openapi.CreateAttendanceRecord201JSONResponse(record), nil
}

// GetAttendanceSpec serves the service's own contract, rendered as YAML from the spec
// oapi-codegen embedded in the generated code. Rendering it from there
// rather than from a second copy of api/openapi.yaml means the two can never
// drift: `make guard-generated` already guarantees the embedded spec matches
// the file on disk.
func (h *Records) GetAttendanceSpec(
	_ context.Context,
	_ openapi.GetAttendanceSpecRequestObject,
) (openapi.GetAttendanceSpecResponseObject, error) {
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

	return openapi.GetAttendanceSpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(specYAML),
		ContentLength: int64(len(specYAML)),
	}, nil
}

// toAPIRecord converts a domain.Record into its generated wire
// representation. domain.Record.Date stays a plain string
// (harness/quality/go/golangci.yml's depguard keeps the domain package
// stdlib-only; the generated openapi_types.Date is squarely an adapter
// concern) - date: date in the contract (api/openapi.yaml) is what makes
// oapi-codegen emit openapi_types.Date for the wire type instead, so this
// is where the two are reconciled. item.Date is only ever "2026-04-01",
// one of the seed fixtures (memory.go) or a string CreateAttendanceRecord
// itself already round-tripped through openapi_types.Date's own
// UnmarshalJSON, so time.Parse failing here would mean the store holds a
// date that never should have gotten in - not something a caller can
// usefully recover from.
func toAPIRecord(item domain.Record) (openapi.Record, error) {
	date, err := time.Parse(openapi_types.DateFormat, item.Date)
	if err != nil {
		return openapi.Record{}, fmt.Errorf("parsing stored date %q: %w", item.Date, err)
	}

	return openapi.Record{
		Id:       item.ID,
		Employee: item.Employee,
		Kind:     openapi.RecordKind(item.Kind),
		Date:     openapi_types.Date{Time: date},
	}, nil
}
