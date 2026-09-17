// Package jev implements usecase.Picker over TypeSafe's Jev API
// (POST /v1/systemone): a second, hosted stand-in for
// internal/adapter/planner/pick's own local Picker, selected by
// ORCHESTRA_PICKER=jev (pkg/app). It sends the shortlist and the three
// fixed built-ins as one "choice" question and reads Jev's own judged
// confidence back as S4's Ambiguous, rather than the local picker's
// "ambiguous" text token.
//
// It also implements usecase.Gate (gate.go), the v3 trial's "noul
// refusal gate" (docs/measurements/jev-picker-v3.md), selected by
// ORCHESTRA_GATE=jev - a separate call over the same endpoint, asked
// before the pick, judging whether a question is answerable at all.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// pickPath is the one endpoint this package calls.
const pickPath = "/v1/systemone"

// modelName is the model this adapter always asks Jev for - the only
// model verified against the contract in the smoke test this adapter was
// built from.
const modelName = "jev-latest"

// requestTimeout bounds one Pick call end to end, retries included: the
// API's own measured latency is ~600ms, so 10s is generous room for the
// two retries below, not a budget this adapter expects to spend.
const requestTimeout = 10 * time.Second

// maxErrorDetailBytes bounds how much of a failing response body is
// folded into an error, the same tradeoff
// internal/adapter/planner/chat and internal/adapter/invoker/http make.
const maxErrorDetailBytes = 4096

// maxRetries is how many times a 429 or 529 response is retried before
// giving up: two, at retryBackoffs' delays, per the contract this
// adapter was built from.
const maxRetries = 2

// statusOverloaded is Jev's 529, not one of net/http's own named
// constants.
const statusOverloaded = 529

// retryBackoff1 and retryBackoff2 are the two retry delays: 1s, then 3s.
const (
	retryBackoff1 = time.Second
	retryBackoff2 = 3 * time.Second
)

// ErrRequestFailed is wrapped into the error returned when Jev answers a
// request with a non-2xx status, after any retries this package makes on
// its own.
var ErrRequestFailed = errors.New("jev request failed")

// wireRequest is the JSON body POST /v1/systemone expects.
type wireRequest struct {
	State     string                  `json:"state"`
	Model     string                  `json:"model"`
	Questions map[string]wireQuestion `json:"questions"`
}

// wireQuestion is one entry of wireRequest.Questions: this adapter only
// ever sends the one named "pick". Criteria is `any` rather than
// map[string]string because CriteriaV2 sends a criterionV2 object (per
// docs.typesafe.ai/primitives/choice's own object form) as each entry's
// value instead of CriteriaV1's plain string - mapping.go's criteriaFor
// and criteriaForV2 are the only two callers, and each builds a map of
// one concrete value type, never a mix of the two within one request.
type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria"`
}

// wireResponse is the JSON body POST /v1/systemone answers with.
type wireResponse struct {
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Usage   wireUsage             `json:"usage"`
}

// wireAnswer is one entry of wireResponse.Answers.
type wireAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// wireUsage is the token accounting Jev returns alongside its answer -
// what Picker.Pick's own "pick completed" log line reports as
// pick_input_tokens and pick_output_tokens.
type wireUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// statusError is a non-2xx response, folding in as much of the body as
// maxErrorDetailBytes allows. It never carries the request's own
// Authorization header, so an error built from it never leaks the API
// key.
type statusError struct {
	status int
	detail []byte
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%v: status %d: %s", ErrRequestFailed, e.status, e.detail)
}

func (e *statusError) Unwrap() error { return ErrRequestFailed }

// retryable reports whether this status is worth retrying: 429 (rate
// limited) or 529 (overloaded), per the contract this adapter was built
// from.
func (e *statusError) retryable() bool {
	return e.status == http.StatusTooManyRequests || e.status == statusOverloaded
}

// client sends one wireRequest to one Jev deployment and returns its
// wireResponse, retrying a 429 or 529 answer per backoffs.
type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	// backoffs is the delay before each retry attempt: retryBackoff1 and
	// retryBackoff2 in production, shrunk by client_internal_test.go's
	// retry tests so they do not pay the real delays.
	backoffs []time.Duration
}

// newClient builds a client for baseURL. A nil httpClient defaults to
// http.DefaultClient.
func newClient(baseURL, apiKey string, httpClient *http.Client) *client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		apiKey:   apiKey,
		http:     httpClient,
		backoffs: []time.Duration{retryBackoff1, retryBackoff2},
	}
}

// pick sends req, retrying a 429/529 answer up to maxRetries times before
// returning the last error.
func (c *client) pick(ctx context.Context, req wireRequest) (wireResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return wireResponse{}, fmt.Errorf("encoding jev request: %w", err)
	}

	respBody, err := c.post(ctx, body)
	if err != nil {
		return wireResponse{}, err
	}

	var wire wireResponse
	if err := json.Unmarshal(respBody, &wire); err != nil {
		return wireResponse{}, fmt.Errorf("decoding jev response: %w", err)
	}

	return wire, nil
}

// gate sends req to the same POST /v1/systemone endpoint as pick, with
// its own wire shape (gateWireRequest's object state, gateWireResponse's
// "noul" answer) - see gate.go. Retrying and error handling are shared
// with pick through post, below; only the request/response JSON types
// differ (mapping.go's buildRequest/wireRequest/wireResponse vs. gate.go's
// buildGateRequest/gateWireRequest/gateWireResponse).
func (c *client) gate(ctx context.Context, req gateWireRequest) (gateWireResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return gateWireResponse{}, fmt.Errorf("encoding jev gate request: %w", err)
	}

	respBody, err := c.post(ctx, body)
	if err != nil {
		return gateWireResponse{}, err
	}

	var wire gateWireResponse
	if err := json.Unmarshal(respBody, &wire); err != nil {
		return gateWireResponse{}, fmt.Errorf("decoding jev gate response: %w", err)
	}

	return wire, nil
}

// post sends body to pickPath, retrying a 429/529 answer up to maxRetries
// times before returning the last error - the shared retry/backoff/status
// core pick and gate both build their own typed request/response around,
// so this package's one retry policy lives in exactly one place.
func (c *client) post(ctx context.Context, body []byte) ([]byte, error) {
	var lastErr error

	for attempt := 0; ; attempt++ {
		respBody, err := c.doOnce(ctx, body)
		if err == nil {
			return respBody, nil
		}

		lastErr = err

		var se *statusError
		if !errors.As(err, &se) || !se.retryable() || attempt >= maxRetries {
			return nil, lastErr
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("jev request: %w", ctx.Err())
		case <-time.After(c.backoffs[attempt]):
		}
	}
}

// doOnce sends body once and returns a successful response's raw bytes,
// undecoded - pick and gate each decode into their own wire type.
func (c *client) doOnce(ctx context.Context, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+pickPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building jev request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", httpReq.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		detail, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorDetailBytes))
		if readErr != nil {
			detail = nil
		}

		return nil, &statusError{status: resp.StatusCode, detail: detail}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading jev response: %w", err)
	}

	return respBody, nil
}
