package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
)

// requestTimeout bounds one Messages API call, mirroring
// internal/adapter/planner/chat.Client's own requestTimeout (that
// constant's own doc comment: generous for a model that may think or call
// a tool, not unbounded).
const requestTimeout = 120 * time.Second

// maxErrorDetailBytes bounds how much of a failing response body is folded
// into an error, mirroring internal/adapter/planner/chat's own constant of
// the same name.
const maxErrorDetailBytes = 4096

// ErrRequestFailed is wrapped into the error returned when the endpoint
// answers with a non-2xx status.
var ErrRequestFailed = errors.New("anthropic messages request failed")

// Client sends Messages API requests to one Anthropic endpoint (or, in a
// test, a fixture server standing in for one over Config.BaseURL).
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client from cfg.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: requestTimeout}}
}

// Client satisfies chat.Completer, so it drops into toolcall.New/pick.New
// exactly where a *chat.Client does.
var _ chat.Completer = (*Client)(nil)

// Complete sends req to the Anthropic Messages API and maps its response
// onto the shared chat.Response shape (fromWireResponse's own doc
// comment). Every call logs, at info, the model, input and output token
// counts, latency and stop reason - "a run's log must be enough to compute
// what it cost" - before mapping the response, so a refusal (which
// Complete then returns as an error) is still logged with its own token
// cost and latency rather than silently swallowed into just an error.
func (c *Client) Complete(ctx context.Context, req *chat.Request) (chat.Response, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}

	wire, err := toWireRequest(model, req)
	if err != nil {
		return chat.Response{}, err
	}

	body, err := json.Marshal(wire)
	if err != nil {
		return chat.Response{}, fmt.Errorf("encoding anthropic messages request: %w", err)
	}

	httpReq, err := c.newHTTPRequest(ctx, body)
	if err != nil {
		return chat.Response{}, err
	}

	start := time.Now()

	httpResp, err := c.http.Do(httpReq)

	latency := time.Since(start)
	if err != nil {
		return chat.Response{}, fmt.Errorf("requesting %s: %w", httpReq.URL, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= http.StatusMultipleChoices {
		return chat.Response{}, requestError(httpReq, httpResp)
	}

	var wireResp wireResponseBody
	if err := json.NewDecoder(httpResp.Body).Decode(&wireResp); err != nil {
		return chat.Response{}, fmt.Errorf("decoding anthropic messages response: %w", err)
	}

	slog.Default().InfoContext(ctx, "anthropic chat completion",
		slog.String("model", model),
		slog.Int("input_tokens", wireResp.Usage.InputTokens),
		slog.Int("output_tokens", wireResp.Usage.OutputTokens),
		slog.Int64("latency_ms", latency.Milliseconds()),
		slog.String("stop_reason", wireResp.StopReason))

	return fromWireResponse(&wireResp)
}

// newHTTPRequest builds the POST /v1/messages request for body, with the
// three headers the Messages API requires: "x-api-key" (never logged - see
// Config.APIKey's own doc comment), "anthropic-version" and
// "content-type".
func (c *Client) newHTTPRequest(ctx context.Context, body []byte) (*http.Request, error) {
	baseURL := c.cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/v1/messages", bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("building anthropic messages request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.cfg.APIKey)
	httpReq.Header.Set("Anthropic-Version", anthropicVersion)

	return httpReq, nil
}

// requestError builds the error for a non-2xx response, folding in as
// much of the body as maxErrorDetailBytes allows - mirroring
// internal/adapter/planner/chat's own requestError.
func requestError(req *http.Request, resp *http.Response) error {
	detail, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorDetailBytes))
	if readErr != nil {
		detail = nil
	}

	return fmt.Errorf("%w: %s -> %d: %s", ErrRequestFailed, req.URL, resp.StatusCode, detail)
}

// BuildRequestBody marshals the Messages API request body Complete would
// send for model and req, without sending it. This is what a token-count
// helper needs: POST /v1/messages/count_tokens takes the same headers
// (x-api-key, anthropic-version, content-type) and the same body minus
// "max_tokens" - a caller wanting to count tokens can json.Unmarshal this
// result, delete "max_tokens", and POST that to count_tokens with the same
// free endpoint, no billed completion involved.
func BuildRequestBody(model string, req *chat.Request) ([]byte, error) {
	wire, err := toWireRequest(model, req)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encoding anthropic messages request: %w", err)
	}

	return body, nil
}
