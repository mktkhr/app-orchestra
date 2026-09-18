package anthropic

import (
	"errors"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
)

// ErrMissingModel is returned when neither chat.Request.Model nor the
// Client's own configured Config.Model names a model - the Anthropic
// Messages API has no default to fall back to (unlike
// internal/adapter/planner/chat.Client, some OpenAI-compatible endpoints
// tolerate an empty "model" behind a router; this one does not).
var ErrMissingModel = errors.New("anthropic: no model configured")

// ErrMissingMaxTokens is returned when req.MaxTokens is nil. The Messages
// API requires "max_tokens" on every request; every existing caller
// (toolcall.Planner, pick.Picker) always sets it (chat.MaxTokens,
// pickMaxTokens), so this only fires for a caller this adapter was not
// written to expect - failing loudly here is cheaper than a 400 with a
// vague body.
var ErrMissingMaxTokens = errors.New("anthropic: request has no max_tokens")

// wireMessage is one entry of wireRequestBody's "messages" - only ever a
// plain string "content" here, never the content-block array form the
// Messages API also accepts, because this adapter never sends more than
// one user turn and never needs the model to see its own prior response.
type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// wireTool is one entry of wireRequestBody's "tools": the Anthropic
// Messages API's own shape for a callable function, built from
// chat.ToolDefinition.Function - chat.FunctionDefinition.Strict has no
// Anthropic equivalent and is silently dropped (OpenAI's own "strict
// mode" toggle; the Messages API always validates a tool_use block's
// "input" against "input_schema").
type wireTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// wireThinking is wireRequestBody's "thinking" field, sent only for a
// model whose behavior says it thinks by default (params.go).
type wireThinking struct {
	Type string `json:"type"`
}

// wireRequestBody is the JSON actually sent to POST /v1/messages, and -
// via BuildRequestBody, with "max_tokens" deleted by the caller - what
// POST /v1/messages/count_tokens takes.
type wireRequestBody struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	System      string        `json:"system,omitempty"`
	Messages    []wireMessage `json:"messages"`
	Tools       []wireTool    `json:"tools,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	Thinking    *wireThinking `json:"thinking,omitempty"`
	// ToolChoice is never populated: chat.Request itself carries no
	// tool_choice field today (every existing caller leaves the endpoint
	// to its own "auto" default), so there is nothing for toWireRequest
	// to map. If a future caller ever needs to force a tool call, note
	// before adding it here: the Messages API's forced tool_choice
	// ({"type":"any"} or {"type":"tool","name":...}) is rejected on some
	// models when combined with extended thinking - it cannot simply be
	// turned on for every model behaviorFor knows about.
}

// toWireRequest builds the Messages API request body for model and req -
// the one function both Client.Complete and BuildRequestBody go through,
// so "what goes on the wire" has exactly one definition.
func toWireRequest(model string, req *chat.Request) (wireRequestBody, error) {
	if model == "" {
		return wireRequestBody{}, ErrMissingModel
	}

	if req.MaxTokens == nil {
		return wireRequestBody{}, ErrMissingMaxTokens
	}

	system, messages := splitSystem(req.Messages)

	wire := wireRequestBody{
		Model:     model,
		MaxTokens: *req.MaxTokens,
		System:    system,
		Messages:  messages,
		Tools:     toWireTools(req.Tools),
	}

	behavior := behaviorFor(model)

	if behavior.sendTemperature && req.Temperature != nil {
		wire.Temperature = req.Temperature
	}

	if behavior.thinkingDisabled {
		wire.Thinking = &wireThinking{Type: "disabled"}
	}

	return wire, nil
}

// splitSystem pulls every "system"-role chat.Message out of messages into
// the Messages API's own single "system" string (its own top-level field,
// never a "messages" entry - this package's own doc comment), joining more
// than one with a blank line, and returns every other message, in order,
// as wireMessage. No existing caller (toolcall.buildMessages,
// pick.Picker.Pick) ever sends more than one system message and one user
// message, but this does not assume that.
func splitSystem(messages []chat.Message) (string, []wireMessage) {
	var systemParts []string

	wireMessages := make([]wireMessage, 0, len(messages))

	for _, m := range messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)

			continue
		}

		wireMessages = append(wireMessages, wireMessage{Role: m.Role, Content: m.Content})
	}

	return strings.Join(systemParts, "\n\n"), wireMessages
}

// toWireTools maps chat.Request.Tools' OpenAI function-calling shape
// ({"type":"function","function":{name,description,parameters}}) onto the
// Messages API's own flat one ({name,description,input_schema}). nil, not
// an empty slice, for no tools at all (pick.Picker never sends any) - the
// same "absent, not empty" convention Request's own omitempty tag relies
// on for the field to disappear from the wire body entirely.
func toWireTools(tools []chat.ToolDefinition) []wireTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]wireTool, len(tools))
	for i, t := range tools {
		out[i] = wireTool{Name: t.Function.Name, Description: t.Function.Description, InputSchema: t.Function.Parameters}
	}

	return out
}
