package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	invokerhttp "github.com/mktkhr/app-orchestra/services/platform/internal/adapter/invoker/http"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

func listEndpoint() *domain.Endpoint {
	return &domain.Endpoint{
		Service:     "inventory",
		OperationID: "ListInventoryItems",
		Method:      domain.MethodGet,
		Path:        "/api/inventory/items",
		Parameters: []domain.Parameter{
			{Name: "status", In: "query", Schema: domain.Schema{Type: domain.SchemaTypeString}},
		},
	}
}

func getEndpoint() *domain.Endpoint {
	return &domain.Endpoint{
		Service:     "inventory",
		OperationID: "GetInventoryItem",
		Method:      domain.MethodGet,
		Path:        "/api/inventory/items/{id}",
		Parameters: []domain.Parameter{
			{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
		},
	}
}

func createEndpoint() *domain.Endpoint {
	return &domain.Endpoint{
		Service:     "inventory",
		OperationID: "CreateInventoryItem",
		Method:      "POST",
		Path:        "/api/inventory/items",
		RequestBody: &domain.Schema{
			Type:       domain.SchemaTypeObject,
			Properties: map[string]domain.Schema{"name": {Type: domain.SchemaTypeString}},
		},
	}
}

func csvEndpoint() *domain.Endpoint {
	return &domain.Endpoint{
		Service:     "inventory",
		OperationID: "ReplaceInventoryCsv",
		Method:      "PUT",
		Path:        "/api/inventory/csv",
		RequestBody: &domain.Schema{Type: domain.SchemaTypeString},
	}
}

// writeJSON writes body as the response, reporting a test failure (without
// using require, which testifylint's go-require forbids inside an HTTP
// handler goroutine) if the write itself fails.
func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("writing fixture response: %v", err)
	}
}

func TestInvokeSendsQueryParametersAndDecodesTheResponse(t *testing.T) {
	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(t, w, `{"items":[]}`)
	}))
	defer server.Close()

	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

	data, err := inv.Invoke(t.Context(), listEndpoint(), map[string]any{"status": "allocated"})

	require.NoError(t, err)
	assert.Equal(t, "status=allocated", gotQuery)
	assert.Equal(t, map[string]any{"items": []any{}}, data)
}

func TestInvokeSubstitutesPathParameters(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, `{"id":"abc"}`)
	}))
	defer server.Close()

	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

	_, err := inv.Invoke(t.Context(), getEndpoint(), map[string]any{"id": "abc"})

	require.NoError(t, err)
	assert.Equal(t, "/api/inventory/items/abc", gotPath)
}

func TestInvokeSendsAnObjectRequestBodyAsJSON(t *testing.T) {
	var gotBody map[string]any
	var decodeErr error

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		decodeErr = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if _, err := w.Write([]byte(`{"id":"1","name":"widget"}`)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	defer server.Close()

	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

	data, err := inv.Invoke(t.Context(), createEndpoint(), map[string]any{"name": "widget"})

	require.NoError(t, err)
	require.NoError(t, decodeErr)
	assert.Equal(t, map[string]any{"name": "widget"}, gotBody)
	assert.Equal(t, map[string]any{"id": "1", "name": "widget"}, data)
}

func TestInvokeSendsANonObjectRequestBodyUnderTheBodyProperty(t *testing.T) {
	var gotBody map[string]any
	var decodeErr error

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decodeErr = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

	data, err := inv.Invoke(t.Context(), csvEndpoint(), map[string]any{"body": "a,b,c"})

	require.NoError(t, err)
	require.NoError(t, decodeErr)
	assert.Equal(t, map[string]any{"body": "a,b,c"}, gotBody)
	assert.Equal(t, map[string]any{}, data)
}

func TestInvokeIgnoresNilArguments(t *testing.T) {
	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(t, w, `{}`)
	}))
	defer server.Close()

	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

	_, err := inv.Invoke(t.Context(), listEndpoint(), map[string]any{"status": nil})

	require.NoError(t, err)
	assert.Empty(t, gotQuery)
}

func TestInvokeUnknownServiceFails(t *testing.T) {
	inv := invokerhttp.New(nil, nil)

	_, err := inv.Invoke(t.Context(), listEndpoint(), nil)

	require.Error(t, err)
	require.ErrorIs(t, err, invokerhttp.ErrUnknownService)
}

// TestInvokeServiceErrorStatusCarriesATypedServiceError is the dev-stack
// defect's adapter half (2026-09-16, /tmp/orchestra-platform.log
// 2026-09-16T20:52:15): every non-2xx response yields a usecase.ServiceError
// alongside ErrServiceError - Status always the response's own, Message its
// body's own {"message": "..."} field when it gave one and "" when the
// body was empty or not JSON shaped that way (a service is not required to
// answer its errors this way).
//
// A 5xx is shaped exactly like a 4xx's ServiceError here - same Status
// field, same optional Message, not a distinct error shape - because it is
// Orchestrator.resultForInvokeError, not this adapter, that tells "the
// service answered no" (4xx) apart from "the service or platform is at
// fault" (5xx), by reading Status off the one type both cases share.
func TestInvokeServiceErrorStatusCarriesATypedServiceError(t *testing.T) {
	tests := map[string]struct {
		status  int
		body    string
		wantErr usecase.ServiceError
	}{
		"404 with a JSON message body": {
			status:  http.StatusNotFound,
			body:    `{"message":"no item"}`,
			wantErr: usecase.ServiceError{Status: http.StatusNotFound, Message: "no item"},
		},
		"404 with a non-JSON body carries no message": {
			status:  http.StatusNotFound,
			body:    `not json`,
			wantErr: usecase.ServiceError{Status: http.StatusNotFound, Message: ""},
		},
		"503 carries the same typed error": {
			status:  http.StatusServiceUnavailable,
			body:    `{"message":"try again later"}`,
			wantErr: usecase.ServiceError{Status: http.StatusServiceUnavailable, Message: "try again later"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)

				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Errorf("writing fixture response: %v", err)
				}
			}))
			defer server.Close()

			inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

			_, err := inv.Invoke(t.Context(), getEndpoint(), map[string]any{"id": "missing"})

			require.Error(t, err)
			require.ErrorIs(t, err, invokerhttp.ErrServiceError)

			var svcErr usecase.ServiceError
			require.ErrorAs(t, err, &svcErr)
			assert.Equal(t, tt.wantErr, svcErr)
		})
	}
}

func TestInvokeUnreachableServiceFails(t *testing.T) {
	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: "http://127.0.0.1:0"}}, nil)

	_, err := inv.Invoke(t.Context(), listEndpoint(), nil)

	require.Error(t, err)
}

func TestInvokeMalformedResponseFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, `not json`)
	}))
	defer server.Close()

	inv := invokerhttp.New([]invokerhttp.Service{{Name: "inventory", URL: server.URL}}, nil)

	_, err := inv.Invoke(t.Context(), listEndpoint(), nil)

	require.Error(t, err)
}
