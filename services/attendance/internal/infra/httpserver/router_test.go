package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/repository"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/infra/httpserver"
)

func TestNewRouterServesRecords(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewRecords(repository.NewMemory()))
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/attendance/records", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNewRouterRejectsUnknownRoute(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewRecords(repository.NewMemory()))
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/does-not-exist", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewRouterServesOpenAPISpec(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewRecords(repository.NewMemory()))
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.yaml", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))
}
