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
// Client's configured default (see Config.Model).
type Request struct {
	Model          string
	Messages       []Message
	Tools          []ToolDefinition
	ResponseFormat *ResponseFormat
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

// Complete sends req and returns the endpoint's one choice.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}

	body, err := json.Marshal(wireRequest{
		Model:          model,
		Messages:       req.Messages,
		Tools:          req.Tools,
		ResponseFormat: req.ResponseFormat,
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
