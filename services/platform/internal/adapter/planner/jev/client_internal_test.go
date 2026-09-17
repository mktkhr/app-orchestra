package jev

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastBackoffs is what newTestClient gives its client instead of the
// real retryBackoff1/retryBackoff2, so the retry tests below do not pay
// the real 1s/3s delays this package uses in production.
var fastBackoffs = []time.Duration{time.Millisecond, time.Millisecond}

// newTestClient builds a client against server with fastBackoffs.
func newTestClient(server *httptest.Server) *client {
	c := newClient(server.URL, "test-key", nil)
	c.backoffs = fastBackoffs

	return c
}

func choiceResponseBody(choice string) string {
	return `{"model":"jev-1.13.0","answers":{"pick":{"type":"choice","choice":"` + choice +
		`","confidence":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`
}

// noulResponseBodyInternal is a fixed "noul" answer body - gate's own
// counterpart to choiceResponseBody, used only to exercise client.gate's
// shared retry core (post/doOnce), not to assert on the noul value
// itself (gate_test.go does that, through the exported Gate type).
const noulResponseBodyInternal = `{"model":"jev-1.13.0","answers":{"gate":{"type":"noul","noul":0.9}},` +
	`"usage":{"input_tokens":1,"output_tokens":1}}`

// TestClientPickRetries429ThenSucceeds is the contract's own "429, then
// retried, then success": two failures at 429, the third attempt answers
// 200, and pick returns that answer rather than an error.
func TestClientPickRetries429ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) <= maxRetries {
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(choiceResponseBody("none"))); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	c := newTestClient(server)

	resp, err := c.pick(context.Background(), wireRequest{})
	require.NoError(t, err)
	assert.Equal(t, "none", resp.Answers["pick"].Choice)
	assert.Equal(t, int32(maxRetries+1), attempts.Load())
}

// TestClientPickGivesUpAfterMaxRetries429sInARow is the contract's own
// "429 x3 -> error": every attempt (the first plus maxRetries retries)
// answers 429, and pick reports an error rather than retrying forever.
func TestClientPickGivesUpAfterMaxRetries429sInARow(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(server)

	_, err := c.pick(context.Background(), wireRequest{})
	require.ErrorIs(t, err, ErrRequestFailed)
	assert.Equal(t, int32(maxRetries+1), attempts.Load())
}

// TestClientPickDoesNotRetryANonRetryableStatus documents that a 401 (or
// any status other than 429/529) fails on the first attempt: only a
// rate-limit or overload response is worth retrying.
func TestClientPickDoesNotRetryANonRetryableStatus(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(server)

	_, err := c.pick(context.Background(), wireRequest{})
	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load())
}

// TestClientGateRetries429ThenSucceeds mirrors TestClientPickRetries429
// ThenSucceeds for client.gate: the same shared retry core (post/doOnce)
// pick already exercises, proven here against gate's own request/response
// types too.
func TestClientGateRetries429ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) <= maxRetries {
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(noulResponseBodyInternal)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	c := newTestClient(server)

	resp, err := c.gate(context.Background(), gateWireRequest{})
	require.NoError(t, err)
	assert.InDelta(t, 0.9, resp.Answers["gate"].Noul, 0.0001)
	assert.Equal(t, int32(maxRetries+1), attempts.Load())
}

// TestClientGateNonTwoHundredIsAnError mirrors TestClientPickDoesNotRetry
// ANonRetryableStatus for client.gate: a non-retryable status fails on
// the first attempt.
func TestClientGateNonTwoHundredIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(server)

	_, err := c.gate(context.Background(), gateWireRequest{})
	require.Error(t, err)
}

// TestStatusErrorNeverCarriesTheAPIKey guards the "401 -> error naming the
// status without the key" contract at the type that builds the message:
// statusError is built from the response alone, so the key - which never
// appears in a response body a well-behaved server would send back - has
// no path into it either way, but the API key used to build the request
// must still never show up in the error text.
func TestStatusErrorNeverCarriesTheAPIKey(t *testing.T) {
	err := &statusError{status: http.StatusUnauthorized, detail: []byte("invalid credentials")}
	assert.Contains(t, err.Error(), "401")
	assert.NotContains(t, err.Error(), "super-secret-key")
}
