package httpserver_test

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/infra/httpserver"
)

func TestNewServerSetsAddr(t *testing.T) {
	server := httpserver.NewServer(9192, http.NewServeMux())

	assert.Equal(t, ":9192", server.Addr)
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

// Binding a port already held by another listener makes ListenAndServe fail
// for a reason other than a clean shutdown, exercising Run's error-wrapping
// path.
func TestRunWrapsListenError(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	_, portStr, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	server := httpserver.NewServer(port, http.NewServeMux())

	err = httpserver.Run(server)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "serving http")
}
