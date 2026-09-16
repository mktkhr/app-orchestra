package handler_test

import (
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/repository"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/domain"
)

// fakeStore is a minimal recordStore double (handler.NewRecords accepts
// any value with this method set, whether or not the interface itself is
// exported) - here only to make Create return a record whose Date could
// never come from a real request (CreateAttendanceRecord's own Date
// argument is already a parsed openapi_types.Date), so
// TestRecordsCreateAttendanceRecordReportsAnUnparsableStoredDate can drive
// toAPIRecord's error path from the create side, the way
// TestRecordsGetAttendanceRecordReportsAnUnparsableStoredDate already
// drives it from the get side.
type fakeStore struct {
	created domain.Record
}

func (f *fakeStore) List(_ *domain.Kind) []domain.Record { return nil }

func (f *fakeStore) Get(_ string) (domain.Record, bool) { return domain.Record{}, false }

func (f *fakeStore) Create(_ domain.NewRecord) domain.Record { return f.created }

func TestRecordsListAttendanceRecordsReturnsEveryRecordWithoutFilter(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.ListAttendanceRecords(t.Context(), openapi.ListAttendanceRecordsRequestObject{})

	require.NoError(t, err)
	list, ok := resp.(openapi.ListAttendanceRecords200JSONResponse)
	require.True(t, ok)
	assert.Len(t, list.Records, 8)
}

func TestRecordsListAttendanceRecordsFiltersByKind(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())
	kind := openapi.OnCall

	resp, err := h.ListAttendanceRecords(t.Context(), openapi.ListAttendanceRecordsRequestObject{
		Params: openapi.ListAttendanceRecordsParams{Kind: &kind},
	})

	require.NoError(t, err)
	list, ok := resp.(openapi.ListAttendanceRecords200JSONResponse)
	require.True(t, ok)
	require.NotEmpty(t, list.Records)
	for _, item := range list.Records {
		assert.Equal(t, openapi.OnCall, item.Kind)
	}
}

// Substitute and compensatory are distinct arrangements in Japanese labour
// practice; a query for one must not silently return the other.
func TestRecordsListAttendanceRecordsDistinguishesSubstituteAndCompensatory(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())
	substitute := openapi.Substitute
	compensatory := openapi.Compensatory

	substituteResp, err := h.ListAttendanceRecords(t.Context(), openapi.ListAttendanceRecordsRequestObject{
		Params: openapi.ListAttendanceRecordsParams{Kind: &substitute},
	})
	require.NoError(t, err)
	substituteList, ok := substituteResp.(openapi.ListAttendanceRecords200JSONResponse)
	require.True(t, ok)

	compensatoryResp, err := h.ListAttendanceRecords(t.Context(), openapi.ListAttendanceRecordsRequestObject{
		Params: openapi.ListAttendanceRecordsParams{Kind: &compensatory},
	})
	require.NoError(t, err)
	compensatoryList, ok := compensatoryResp.(openapi.ListAttendanceRecords200JSONResponse)
	require.True(t, ok)

	require.NotEmpty(t, substituteList.Records)
	require.NotEmpty(t, compensatoryList.Records)
	for _, item := range substituteList.Records {
		assert.Equal(t, openapi.Substitute, item.Kind)
	}
	for _, item := range compensatoryList.Records {
		assert.Equal(t, openapi.Compensatory, item.Kind)
	}
}

func TestRecordsGetAttendanceRecordReturnsExistingRecord(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.GetAttendanceRecord(t.Context(), openapi.GetAttendanceRecordRequestObject{Id: "att-001"})

	require.NoError(t, err)
	got, ok := resp.(openapi.GetAttendanceRecord200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, "att-001", got.Id)
}

func TestRecordsGetAttendanceRecordReportsMissingRecord(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.GetAttendanceRecord(t.Context(), openapi.GetAttendanceRecordRequestObject{Id: "does-not-exist"})

	require.NoError(t, err)
	_, ok := resp.(openapi.GetAttendanceRecord404JSONResponse)
	assert.True(t, ok)
}

// TestRecordsGetAttendanceRecordReportsAnUnparsableStoredDate covers
// toAPIRecord's own error path (2026-09-16, api/openapi.yaml's date field
// gained format: date, so the wire type is openapi_types.Date, not a bare
// string): a record whose stored Date cannot be parsed as YYYY-MM-DD - not
// reachable through CreateAttendanceRecord, whose own Date argument is
// already a parsed openapi_types.Date by the time it reaches the store,
// but reachable by writing to the store directly, the same way a real
// storage layer's own corruption would surface.
func TestRecordsGetAttendanceRecordReportsAnUnparsableStoredDate(t *testing.T) {
	store := repository.NewMemory()
	created := store.Create(domain.NewRecord{Employee: "テスト社員", Kind: domain.KindDeemed, Date: "not-a-date"})

	h := handler.NewRecords(store)

	_, err := h.GetAttendanceRecord(t.Context(), openapi.GetAttendanceRecordRequestObject{Id: created.ID})

	require.Error(t, err)
}

// TestRecordsListAttendanceRecordsReportsAnUnparsableStoredDate is
// TestRecordsGetAttendanceRecordReportsAnUnparsableStoredDate's own
// equivalent for the list path.
func TestRecordsListAttendanceRecordsReportsAnUnparsableStoredDate(t *testing.T) {
	store := repository.NewMemory()
	store.Create(domain.NewRecord{Employee: "テスト社員", Kind: domain.KindDeemed, Date: "not-a-date"})

	h := handler.NewRecords(store)

	_, err := h.ListAttendanceRecords(t.Context(), openapi.ListAttendanceRecordsRequestObject{})

	require.Error(t, err)
}

// TestRecordsCreateAttendanceRecordReportsAnUnparsableStoredDate drives
// toAPIRecord's error path from CreateAttendanceRecord's own call to it,
// via fakeStore (see its own doc comment).
func TestRecordsCreateAttendanceRecordReportsAnUnparsableStoredDate(t *testing.T) {
	store := &fakeStore{
		created: domain.Record{ID: "att-x", Employee: "テスト社員", Kind: domain.KindDeemed, Date: "not-a-date"},
	}
	h := handler.NewRecords(store)

	_, err := h.CreateAttendanceRecord(t.Context(), openapi.CreateAttendanceRecordRequestObject{
		Body: &openapi.CreateAttendanceRecordJSONRequestBody{
			Employee: "テスト社員",
			Kind:     openapi.Deemed,
			Date:     openapi_types.Date{Time: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		},
	})

	require.Error(t, err)
}

func TestRecordsCreateAttendanceRecordStoresAndReturnsRecord(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.CreateAttendanceRecord(t.Context(), openapi.CreateAttendanceRecordRequestObject{
		Body: &openapi.CreateAttendanceRecordJSONRequestBody{
			Employee: "テスト社員",
			Kind:     openapi.Deemed,
			Date:     openapi_types.Date{Time: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		},
	})

	require.NoError(t, err)
	created, ok := resp.(openapi.CreateAttendanceRecord201JSONResponse)
	require.True(t, ok)
	assert.NotEmpty(t, created.Id)
	assert.Equal(t, "テスト社員", created.Employee)
	assert.Equal(t, openapi.Deemed, created.Kind)
	assert.Equal(t, "2026-06-01", created.Date.String())
}

func TestRecordsGetAttendanceSpecServesParsableYAML(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.GetAttendanceSpec(t.Context(), openapi.GetAttendanceSpecRequestObject{})

	require.NoError(t, err)
	got, ok := resp.(openapi.GetAttendanceSpec200ApplicationyamlResponse)
	require.True(t, ok)

	var doc map[string]any
	require.NoError(t, yaml.NewDecoder(got.Body).Decode(&doc))
	assert.Equal(t, "3.0.3", doc["openapi"])
}
