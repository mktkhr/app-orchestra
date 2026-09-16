// Package acceptance drives the real attendance service object graph
// (pkg/app) behind httptest, over HTTP, the way a client actually would. It
// is a separate module so it cannot reach into the service's internal/ tree.
package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mktkhr/app-orchestra/services/attendance/pkg/app"
)

type record struct {
	ID       string `json:"id"`
	Employee string `json:"employee"`
	Kind     string `json:"kind"`
	Date     string `json:"date"`
}

type recordList struct {
	Records []record `json:"records"`
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler, err := app.New()
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// doJSON sends a request and returns its status code, decoding the JSON body
// into out (when non-nil) before closing it - so the body is always closed
// here, not left for the caller to remember.
func doJSON(t *testing.T, server *httptest.Server, method, path string, body, out any) int {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}

	return resp.StatusCode
}

func TestListAttendanceRecordsFiltersByKind(t *testing.T) {
	server := newTestServer(t)

	var all recordList
	status := doJSON(t, server, http.MethodGet, "/api/attendance/records", nil, &all)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, all.Records, 8)

	var deemed recordList
	status = doJSON(t, server, http.MethodGet, "/api/attendance/records?kind=deemed", nil, &deemed)
	require.Equal(t, http.StatusOK, status)

	require.NotEmpty(t, deemed.Records)
	for _, r := range deemed.Records {
		assert.Equal(t, "deemed", r.Kind)
	}
	assert.Less(t, len(deemed.Records), len(all.Records))
}

// substitute (振替休日, arranged in advance) and compensatory (代休, granted
// afterwards) are different arrangements in Japanese labour practice. This
// is the reason the service exists: a query for one kind must not return the
// other.
func TestListAttendanceRecordsDistinguishesSubstituteAndCompensatory(t *testing.T) {
	server := newTestServer(t)

	var substitute recordList
	status := doJSON(t, server, http.MethodGet, "/api/attendance/records?kind=substitute", nil, &substitute)
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, substitute.Records)
	for _, r := range substitute.Records {
		assert.Equal(t, "substitute", r.Kind)
	}

	var compensatory recordList
	status = doJSON(t, server, http.MethodGet, "/api/attendance/records?kind=compensatory", nil, &compensatory)
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, compensatory.Records)
	for _, r := range compensatory.Records {
		assert.Equal(t, "compensatory", r.Kind)
	}

	substituteIDs := make(map[string]bool, len(substitute.Records))
	for _, r := range substitute.Records {
		substituteIDs[r.ID] = true
	}
	for _, r := range compensatory.Records {
		assert.False(t, substituteIDs[r.ID], "record %s appeared in both substitute and compensatory results", r.ID)
	}
}

func TestGetAttendanceRecordReturnsExistingRecord(t *testing.T) {
	server := newTestServer(t)

	var got record
	status := doJSON(t, server, http.MethodGet, "/api/attendance/records/att-001", nil, &got)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "att-001", got.ID)
}

func TestGetAttendanceRecordReportsMissingRecord(t *testing.T) {
	server := newTestServer(t)

	// att-999 matches the contract's own id pattern (`^att-[0-9]+$`,
	// services/attendance/api/openapi.yaml - added for TODO.md's
	// "real-attendance-detail", read by the platform's usecase.idAffinity)
	// but names no record the fixture seeds: the shape a real "not found"
	// looks like, as opposed to a malformed id, which the request
	// validation middleware now rejects with 400 before this handler ever
	// runs.
	status := doJSON(t, server, http.MethodGet, "/api/attendance/records/att-999", nil, nil)

	assert.Equal(t, http.StatusNotFound, status)
}

func TestCreateAttendanceRecordAddsRecord(t *testing.T) {
	server := newTestServer(t)

	newRecord := map[string]any{"employee": "テスト社員", "kind": "on_call", "date": "2026-07-01"}

	var created record
	status := doJSON(t, server, http.MethodPost, "/api/attendance/records", newRecord, &created)

	require.Equal(t, http.StatusCreated, status)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "テスト社員", created.Employee)
	assert.Equal(t, "on_call", created.Kind)
	assert.Equal(t, "2026-07-01", created.Date)

	var fetched record
	status = doJSON(t, server, http.MethodGet, "/api/attendance/records/"+created.ID, nil, &fetched)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, created, fetched)
}

func TestOpenAPISpecEndpointReturnsParsableYAML(t *testing.T) {
	server := newTestServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/openapi.yaml", http.NoBody)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/yaml", resp.Header.Get("Content-Type"))

	var doc map[string]any
	require.NoError(t, yaml.NewDecoder(resp.Body).Decode(&doc))
	assert.Equal(t, "3.0.3", doc["openapi"])

	paths, ok := doc["paths"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, paths, "/api/attendance/records")
}
