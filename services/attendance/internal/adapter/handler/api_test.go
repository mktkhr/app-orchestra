package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/repository"
)

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

func TestRecordsCreateAttendanceRecordStoresAndReturnsRecord(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.CreateAttendanceRecord(t.Context(), openapi.CreateAttendanceRecordRequestObject{
		Body: &openapi.CreateAttendanceRecordJSONRequestBody{
			Employee: "テスト社員",
			Kind:     openapi.Deemed,
			Date:     "2026-06-01",
		},
	})

	require.NoError(t, err)
	created, ok := resp.(openapi.CreateAttendanceRecord201JSONResponse)
	require.True(t, ok)
	assert.NotEmpty(t, created.Id)
	assert.Equal(t, "テスト社員", created.Employee)
	assert.Equal(t, openapi.Deemed, created.Kind)
	assert.Equal(t, "2026-06-01", created.Date)
}

func TestRecordsGetSpecServesParsableYAML(t *testing.T) {
	h := handler.NewRecords(repository.NewMemory())

	resp, err := h.GetSpec(t.Context(), openapi.GetSpecRequestObject{})

	require.NoError(t, err)
	got, ok := resp.(openapi.GetSpec200ApplicationyamlResponse)
	require.True(t, ok)

	var doc map[string]any
	require.NoError(t, yaml.NewDecoder(got.Body).Decode(&doc))
	assert.Equal(t, "3.0.3", doc["openapi"])
}
