// Package acceptance drives the real platform object graph (pkg/app) behind
// httptest, over HTTP, the way a client actually would. It is a separate
// module so it cannot reach into the service's internal/ tree.
package acceptance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// TestHealthEndpointReportsOK only ever calls GET /api/health, which is
// exempt from requireSession (docs/specs/auth.md, AC-A-102) - so this
// builds its own Config directly, with just enough (a DBPath -
// pkg/app.ErrMissingDBPath - and the admin password it requires) to build
// a handler at all, rather than the shared newTestApp (helpers_test.go),
// which also signs in for routes that need it.
func TestHealthEndpointReportsOK(t *testing.T) {
	handler, err := app.New(&app.Config{
		DBPath:        filepath.Join(t.TempDir(), "acceptance.db"),
		AdminPassword: adminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/api/health", http.NoBody)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body.Status)
}
