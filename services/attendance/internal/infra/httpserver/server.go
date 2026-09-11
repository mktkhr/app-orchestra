package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// timeout bounds every request the server accepts; the attendance service
// serves no long-lived connections.
const timeout = 15 * time.Second

// NewServer builds an *http.Server bound to port, serving handler.
func NewServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handler,
		ReadHeaderTimeout: timeout,
		ReadTimeout:       timeout,
		WriteTimeout:      timeout,
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
