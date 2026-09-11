package app_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/attendance/pkg/app"
)

func TestNewServesRecords(t *testing.T) {
	h, err := app.New()
	require.NoError(t, err)

	server := httptest.NewServer(h)
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/api/attendance/records", http.NoBody)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
