package app

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
)

// errBoom is a fixture used only to exercise build's error-wrapping path,
// which a valid embedded OpenAPI spec never takes.
var errBoom = errors.New("boom")

func TestBuildWrapsRouterError(t *testing.T) {
	cfg := &Config{
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: "correct horse battery staple",
	}

	_, err := build(cfg, func(openapi.StrictServerInterface, string, httpserver.SessionUsers) (http.Handler, error) {
		return nil, errBoom
	})

	require.Error(t, err)
	require.ErrorIs(t, err, errBoom)
}
