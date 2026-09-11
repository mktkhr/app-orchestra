package httpserver_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
)

func TestNewServerSetsAddr(t *testing.T) {
	server := httpserver.NewServer(9091, http.NewServeMux())

	assert.Equal(t, ":9091", server.Addr)
}

// A server that is already shut down makes ListenAndServe return
// http.ErrServerClosed immediately, without ever binding a port - the
// documented way to exercise Run's clean-shutdown path deterministically.
func TestRunReturnsNilAfterShutdown(t *testing.T) {
	server := httpserver.NewServer(0, http.NewServeMux())

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, server.Shutdown(ctx))

	require.NoError(t, httpserver.Run(server))
}
