package chat

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

// requestTimeout bounds one chat-completions call. Generous, because a
// local model answering with a tool call can be slow on modest hardware,
// but not unbounded - a hung backend must not hang the request it serves.
const requestTimeout = 120 * time.Second

// maxErrorDetailBytes bounds how much of a failing response body is folded
// into an error, the same tradeoff internal/adapter/invoker/http makes.
const maxErrorDetailBytes = 4096

// ErrRequestFailed is wrapped into the error returned when the endpoint
// answers with a non-2xx status.
var ErrRequestFailed = errors.New("chat completion request failed")

// ErrNoChoices is returned when a response decodes but names no choice at
// all - a shape no OpenAI-compatible endpoint is documented to produce,
// but not one the wire format rules out either.
var ErrNoChoices = errors.New("chat completion response had no choices")

// Message is one entry of a chat-completions conversation: a system or
// user turn (Content set), an assistant turn that called a tool
// (ToolCalls set), or a tool's own reply to a call (ToolCallID and Name
// set). This adapter only ever sends system/user turns and reads an
// assistant turn back - Task 10 calls the model exactly once per request
// (D8) - but the tool-result turn is part of the wire format regardless.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is one function call an assistant message made.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall is a ToolCall's payload: the tool's name and its arguments,
// which arrive as a JSON-encoded string (not a nested object) per the
// OpenAI wire format - the caller decodes it.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition is one entry of a Request's Tools: the OpenAI
// chat-completions shape for a callable function.
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition describes one tool's name, purpose and JSON Schema
// parameters. Strict is a pointer so "unset" (let the endpoint decide) is
// distinct from "false" - the tool-calling planner always sets it
// explicitly (see internal/adapter/planner/toolcall), but this package
// does not require that.
type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
}

// ResponseFormat asks the endpoint to constrain its answer, for the JSON
// planner (Task 11) that does not use tool calling at all.
type ResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema map[string]any `json:"json_schema,omitempty"`
}

// Request is one chat-completions call. Model, when empty, uses the
// Client's configured default (see Config.Model). Temperature is a
// pointer so "unset" (let the endpoint decide) is distinct from "0" - both
// planners (internal/adapter/planner/toolcall,
// internal/adapter/planner/jsonmode) always set it explicitly, to 0, for
// every planning call: sent as nothing, llama-server's own default
// applies, and the same 100-question fixture scored 32 and then 16 on one
// axis across two runs with nothing else changed (measured 2026-09-15,
// docs/specs/shortlisting.md) - planning has to be deterministic, or a
// measurement of it means nothing.
type Request struct {
	Model          string
	Messages       []Message
	Tools          []ToolDefinition
	ResponseFormat *ResponseFormat
	Temperature    *float64
	MaxTokens      *int
}

// Zero builds Request.Temperature's fixed value for every planning call
// (defect 2, docs/specs/shortlisting.md - see Request's own doc comment):
// a package-level *float64 would be a mutable global every caller shares
// (gochecknoglobals, harness/quality/go/golangci.yml), so this returns a
// fresh pointer instead - callers write `Temperature: chat.Zero()`.
func Zero() *float64 {
	temperature := 0.0

	return &temperature
}

// planningMaxTokens is Request.MaxTokens's fixed value for every planning
// call (defect 3, docs/specs/shortlisting.md, measured 2026-09-15).
//
// Reproduction: fixture platform, narrowing on, `POST /api/plan
// {"query":"明細を1件確認したい"}`. Narrowing took 9ms + 88ms, then the chat
// completion never returned headers at all; after exactly 120s (the
// client's own requestTimeout) the platform answered 500 "context deadline
// exceeded (Client.Timeout exceeded while awaiting headers)", and
// llama-swap's own log read `POST /v1/chat/completions 499 0 ... 2m0.000s`
// - the model was still generating when the client gave up. 4 of the
// first 42 questions in the 100-question run hit this (a20, b07, b08,
// b12), each costing the full 120s and an error.
//
// Cause: Request sent no `max_tokens` at all, so llama-server's own
// default (`n_predict = -1`, unbounded) applied; at temperature 0
// (defect 2) with twenty strict tool schemas offered, the model can fall
// into a repetition loop with nothing to stop it. Every planning answer is
// short - one tool call or one sentence - so an unbounded budget buys
// nothing and, on a loop, costs the whole timeout for nothing in return.
const planningMaxTokens = 1024

// MaxTokens builds Request.MaxTokens's fixed value for every planning
// call, for the same "mutable global" reason Zero returns a fresh pointer
// rather than sharing one.
func MaxTokens() *int {
	maxTokens := planningMaxTokens

	return &maxTokens
}

// FinishReasonLength is the OpenAI-compatible finish_reason a Response
// carries when the endpoint stopped generating only because it hit
// Request.MaxTokens, not because it produced a complete answer - the
// signal a caller checks to tell a truncated (and so unusable) answer from
// a real one, rather than trying to parse or tool-call-decode it anyway.
const FinishReasonLength = "length"

// PreviewLen bounds how much of a truncated answer's content Preview
// keeps - enough for a log line to show what a repetition loop (see
// planningMaxTokens) looked like, without the log line becoming the loop's
// own output.
const PreviewLen = 200

// Preview truncates s to at most PreviewLen runes, for logging a
// FinishReasonLength answer's content without flooding the log with it.
func Preview(s string) string {
	r := []rune(s)
	if len(r) <= PreviewLen {
		return s
	}

	return string(r[:PreviewLen])
}

// Response is the one choice this package reads back: n=1 is implicit,
// since every planner that uses this transport asks for exactly one
// decision.
type Response struct {
	Message      Message
	FinishReason string
}

// wireRequest is the JSON actually sent: a Request with Model resolved and
// its zero-value fields (Tools, ResponseFormat) omitted rather than sent
// as null, which some OpenAI-compatible endpoints reject.
type wireRequest struct {
	Model          string           `json:"model"`
	Messages       []Message        `json:"messages"`
	Tools          []ToolDefinition `json:"tools,omitempty"`
	ResponseFormat *ResponseFormat  `json:"response_format,omitempty"`
	Temperature    *float64         `json:"temperature,omitempty"`
	MaxTokens      *int             `json:"max_tokens,omitempty"`
}

// wireResponse is the JSON actually read back: only the one choice's
// message and finish reason matter here.
type wireResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
}

// Client sends chat-completions requests to one OpenAI-compatible
// endpoint.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client from cfg.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: requestTimeout}}
}

// Complete sends req and returns the endpoint's one choice. req is a
// pointer, not the value every caller used to pass directly, because
// adding Temperature (defect 2, docs/specs/shortlisting.md) grew Request
// past golangci-lint's gocritic hugeParam threshold (80 bytes; see
// harness/quality/go/golangci.yml).
func (c *Client) Complete(ctx context.Context, req *Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}

	body, err := json.Marshal(wireRequest{
		Model:          model,
		Messages:       req.Messages,
		Tools:          req.Tools,
		ResponseFormat: req.ResponseFormat,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
	})
	if err != nil {
		return Response{}, fmt.Errorf("encoding chat completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, strings.TrimSuffix(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body),
	)
	if err != nil {
		return Response{}, fmt.Errorf("building chat completion request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	return c.do(httpReq)
}

// do sends req and decodes the successful response's one choice.
func (c *Client) do(req *http.Request) (Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("requesting %s: %w", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return Response{}, requestError(req, resp)
	}

	var wire wireResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return Response{}, fmt.Errorf("decoding chat completion response: %w", err)
	}

	if len(wire.Choices) == 0 {
		return Response{}, fmt.Errorf("%w: %s", ErrNoChoices, req.URL)
	}

	choice := wire.Choices[0]

	return Response{Message: choice.Message, FinishReason: choice.FinishReason}, nil
}

// requestError builds the error for a non-2xx response, folding in as
// much of the body as maxErrorDetailBytes allows.
func requestError(req *http.Request, resp *http.Response) error {
	detail, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorDetailBytes))
	if readErr != nil {
		detail = nil
	}

	return fmt.Errorf("%w: %s -> %d: %s", ErrRequestFailed, req.URL, resp.StatusCode, detail)
}
