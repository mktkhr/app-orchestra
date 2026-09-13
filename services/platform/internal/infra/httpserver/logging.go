package httpserver

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// responseRecorder wraps an http.ResponseWriter so logInternalServerErrors
// can see, after the wrapped handler has already written its response,
// what status it answered with and - only when that status is a 5xx - the
// body it wrote, without changing what actually reaches the client (every
// Write and WriteHeader call is passed through unchanged).
type responseRecorder struct {
	http.ResponseWriter

	status int
	body   bytes.Buffer
}

// WriteHeader records status before writing it, the same way
// http.ResponseWriter's own zero value means 200 until something else is
// written - see Write below for where that default is applied.
func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write records b when the response is a 5xx - the JSON body every 500
// this codebase writes carries an openapi.ErrorResponse.Message naming the
// error that caused it (session.go's writeSessionError, and every
// *500JSONResponse the generated strict handler writes for plan.go,
// invoke.go, workspace.go and users.go) - before passing it through to the
// real ResponseWriter unchanged. A handler that calls Write without ever
// calling WriteHeader answers 200, the same rule net/http itself follows,
// so status is defaulted here rather than left 0.
func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	if r.status >= http.StatusInternalServerError {
		r.body.Write(b)
	}

	n, err := r.ResponseWriter.Write(b)
	if err != nil {
		return n, fmt.Errorf("writing response: %w", err)
	}

	return n, nil
}

// logInternalServerErrors wraps next so that any 5xx response it writes is
// logged with the body that named its cause, through slog's process-wide
// default logger (cmd/api's main sets it to the same JSON handler it logs
// startup with slog.SetDefault). Before this, a 500 was only ever seen by
// the browser that received it - docs/specs/storage.md, section 1: three
// separate investigations of the same "database is locked" failure each
// read a bare "500" and had to guess, because nothing wrote it down
// anywhere a person could read afterward (S4, AC-S-103).
func logInternalServerErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		if rec.status >= http.StatusInternalServerError {
			slog.Default().ErrorContext(r.Context(), "request failed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.String("error", strings.TrimSpace(rec.body.String())),
			)
		}
	})
}
