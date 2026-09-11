package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// readTimeout bounds how long a client may take to send its request. A
// question is a few hundred bytes; anything slower than this is not a
// browser waiting for an answer.
const readTimeout = 15 * time.Second

// writeTimeout bounds how long the platform may take to produce one. It has
// to exceed the planner's own budget for a single model call
// (internal/adapter/planner/chat, 120s), because that call happens inside
// the request: POST /api/plan asks a model and waits, and a local model
// thinking about an ambiguous question routinely passes fifteen seconds.
//
// It was fifteen seconds, matched to the read side on the assumption that
// the platform served no long-lived connections. Every /api/plan is one.
// The server killed the response mid-flight and the browser saw a dropped
// connection with no status at all, which reads as anything but a timeout -
// it was reported three times as the model being flaky.
const writeTimeout = 150 * time.Second

// NewServer builds an *http.Server bound to port, serving handler.
func NewServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handler,
		ReadHeaderTimeout: readTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
	}
}

// Run starts server and blocks until it stops. A clean shutdown
// (http.ErrServerClosed) is not reported as an error.
func Run(server *http.Server) error {
	err := server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return fmt.Errorf("serving http: %w", err)
}
