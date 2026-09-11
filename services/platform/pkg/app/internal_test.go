package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
)

// errBoom is a fixture used only to exercise build's error-wrapping path,
// which a valid embedded OpenAPI spec never takes.
var errBoom = errors.New("boom")

func TestBuildWrapsRouterError(t *testing.T) {
	_, err := build(&Config{}, func(openapi.StrictServerInterface, string, httpserver.SessionUsers) (http.Handler, error) {
		return nil, errBoom
	})

	require.Error(t, err)
	require.ErrorIs(t, err, errBoom)
}
