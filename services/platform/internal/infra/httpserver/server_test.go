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

func TestNewServerAllowsAnAnswerToOutlastAModelCall(t *testing.T) {
	// POST /api/plan asks a model inside the request, and the planner's own
	// budget for that call is 120s (internal/adapter/planner/chat). A write
	// timeout below it kills the response while the platform is still
	// waiting for the answer, and the browser sees a dropped connection
	// rather than a timeout.
	server := httpserver.NewServer(8080, http.NotFoundHandler())

	assert.Greater(t, server.WriteTimeout, 120*time.Second,
		"the write timeout must outlast one model call")
	assert.Less(t, server.ReadTimeout, server.WriteTimeout,
		"reading a question is not what takes the time")
}
