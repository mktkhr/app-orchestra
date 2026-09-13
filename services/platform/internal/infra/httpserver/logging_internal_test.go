package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withCapturedDefaultLogger points slog's process-wide default at a JSON
// handler writing into a buffer this test can read back, for the duration
// of the test - t.Cleanup restores whatever was there before, so this
// package's tests never leak a logger into another package's test binary
// (each runs its own process) or, within this one, into a test that runs
// after it.
func withCapturedDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer

	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	return &buf
}

// failingHandler answers status with body, the same way writeSessionError
// and every generated *500JSONResponse do for a real 500.
func failingHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)

		if _, err := w.Write([]byte(body)); err != nil {
			// httptest.ResponseRecorder, the only writer these tests use,
			// never errors; nothing meaningful to do with a real one
			// either once the status is already written.
			return
		}
	})
}

func TestLogInternalServerErrorsLogsA500WithItsBody(t *testing.T) {
	buf := withCapturedDefaultLogger(t)

	mw := logInternalServerErrors(failingHandler(
		http.StatusInternalServerError, `{"message":"resolving session: looking up session: database is locked"}`,
	))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "the wrapped response must reach the client unchanged")
	assert.Contains(t, rec.Body.String(), "database is locked",
		"the response body must reach the client unchanged, not be swallowed by the logging wrapper")

	logged := buf.String()
	require.NotEmpty(t, logged, "a 500 must be logged")
	assert.Contains(t, logged, "database is locked", "the log line must carry the error that caused the 500")
	assert.Contains(t, logged, "/api/workspaces")
}

func TestLogInternalServerErrorsIgnoresSuccessfulResponses(t *testing.T) {
	buf := withCapturedDefaultLogger(t)

	mw := logInternalServerErrors(okNext)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, buf.String(), "a non-5xx response must not be logged")
}

func TestLogInternalServerErrorsIgnoresA400(t *testing.T) {
	buf := withCapturedDefaultLogger(t)

	mw := logInternalServerErrors(failingHandler(http.StatusBadRequest, `{"message":"bad request"}`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, buf.String(), "a 4xx response is a client's mistake, not a bug this exists to surface")
}

// TestLogInternalServerErrorsDefaultsToOKWhenWriteHeaderIsNeverCalled pins
// responseRecorder.Write's own defaulting rule: a handler that only ever
// calls Write, never WriteHeader, answers 200 - the same rule
// http.ResponseWriter itself follows - so this wrapper must not mistake
// that for an unset, "definitely not a 500" status either.
func TestLogInternalServerErrorsDefaultsToOKWhenWriteHeaderIsNeverCalled(t *testing.T) {
	buf := withCapturedDefaultLogger(t)

	mw := logInternalServerErrors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			return
		}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/workspaces", http.NoBody)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, buf.String())
}
