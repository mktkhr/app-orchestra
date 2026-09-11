package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
)

func TestNewRouterServesHealth(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewPlan(nil), handler.NewInvoke()), "")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestNewRouterRejectsUnknownRoute(t *testing.T) {
	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewPlan(nil), handler.NewInvoke()), "")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/does-not-exist", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewRouterServesStaticDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("hello"), 0o600))

	router, err := httpserver.NewRouter(handler.NewAPI(handler.NewHealth(), handler.NewPlan(nil), handler.NewInvoke()), dir)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}
