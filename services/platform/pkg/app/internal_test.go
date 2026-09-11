package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

// errBoom is a fixture used only to exercise build's error-wrapping path,
// which a valid embedded OpenAPI spec never takes.
var errBoom = errors.New("boom")

func TestBuildWrapsRouterError(t *testing.T) {
	_, err := build("", func(openapi.StrictServerInterface, string) (http.Handler, error) {
		return nil, errBoom
	})

	require.Error(t, err)
	require.ErrorIs(t, err, errBoom)
}
