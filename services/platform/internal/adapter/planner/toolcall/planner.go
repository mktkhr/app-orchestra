// Package toolcall implements usecase.Planner by asking a model to call
// one of usecase.ToolsFor's tools, over internal/adapter/planner/chat's
// OpenAI-compatible transport. It is the planner for models that support
// tool calling (D5, docs/specs/orchestration.md); internal/adapter/planner/jsonmode
// (Task 11) is the other half of the same port, for models that do not.
package toolcall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// toolNameAskUser mirrors usecase.AskUserTool's Name.
const toolNameAskUser = "ask_user"

// toolNameListCapabilities mirrors usecase.ListCapabilitiesTool's Name.
const toolNameListCapabilities = "list_capabilities"

// systemPrompt tells the model how to use the catalogue's tools: call one,
// call list_capabilities when the question is about what can be done at
// all, call ask_user when an enum value is ambiguous, or answer nothing
// when nothing fits.
const systemPrompt = "You are given a set of tools, one per operation of a catalogue of " +
	"internal services, plus ask_user and list_capabilities. Read the user's question, in " +
	"Japanese, and either call exactly one tool that answers it, call list_capabilities when the " +
	"question asks what can be done rather than asking to do something, call ask_user when a " +
	"parameter's value cannot be told from the question, or call no tool at all when nothing in " +
	"the catalogue answers the question."

// ErrUnknownOperation is returned when the model calls a tool whose name
// is not any endpoint in the catalogue this Planner was built with - a
// hallucinated operation id, which resolveService cannot resolve to any
// service at all.
var ErrUnknownOperation = errors.New("planner named an operation not in the catalogue")

// Planner implements usecase.Planner: it sends usecase.ToolsFor's tools to
// a model over chat.Client and maps the tool call it makes onto a
// usecase.Decision.
type Planner struct {
	client  *chat.Client
	catalog domain.Catalog
}

var _ usecase.Planner = (*Planner)(nil)

// New builds a Planner. catalog is needed to resolve the service an
// operation id belongs to (see resolveService) - a tool call names only
// the operation, never the service, so the tool-calling wire format alone
// cannot answer that question.
func New(client *chat.Client, catalog domain.Catalog) *Planner {
	return &Planner{client: client, catalog: catalog}
}

// Plan sends query (with answers folded in, see buildMessages) and tools
// to the model, and maps the one tool call it returns - if any - onto a
// Decision.
func (p *Planner) Plan(
	ctx context.Context, query string, answers []usecase.Answer, tools []usecase.Tool,
) (usecase.Decision, error) {
	resp, err := p.client.Complete(ctx, chat.Request{
		Messages: buildMessages(query, answers),
		Tools:    shapeTools(tools),
	})
	if err != nil {
		return usecase.Decision{}, fmt.Errorf("calling chat completion: %w", err)
	}

	if len(resp.Message.ToolCalls) == 0 {
		return usecase.Decision{Kind: usecase.DecisionNone}, nil
	}

	call := resp.Message.ToolCalls[0].Function

	args, err := decodeArguments(call.Arguments)
	if err != nil {
		return usecase.Decision{}, fmt.Errorf("decoding %s arguments: %w", call.Name, err)
	}

	if call.Name == toolNameAskUser {
		return decisionFromAskUser(args), nil
	}

	if call.Name == toolNameListCapabilities {
		return decisionFromListCapabilities(args), nil
	}

	return p.decisionFromCall(call.Name, args)
}

// decodeArguments parses a tool call's Arguments string as a JSON object.
// An empty string - a tool with no parameters, called with none - decodes
// to an empty map rather than an error.
func decodeArguments(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("parsing arguments %q: %w", raw, err)
	}

	return args, nil
}

// decisionFromAskUser builds a DecisionAsk from ask_user's arguments.
// Options is read and carried onto Decision.Options because Decision
// declares the field and a caller may want to see what the model proposed,
// but Orchestrator.ask (internal/usecase/orchestrator.go) never trusts it -
// it rebuilds the options a person is offered from the catalogue's own
// enum instead, since a model can list candidate values that do not exist.
func decisionFromAskUser(args map[string]any) usecase.Decision {
	return usecase.Decision{
		Kind:        usecase.DecisionAsk,
		Service:     stringArg(args, "service"),
		OperationID: stringArg(args, "operationId"),
		Question:    stringArg(args, "question"),
		Param:       stringArg(args, "param"),
		Options:     optionsArg(args, "options"),
	}
}

// decisionFromListCapabilities builds a DecisionListCapabilities from
// list_capabilities' arguments. Unlike decisionFromCall, this never
// touches resolveService: list_capabilities is not an operation id in the
// catalogue (usecase.ListCapabilitiesTool, like ask_user, is not derived
// from any service's spec), so looking it up there would only ever fail.
// Its optional "service" argument is carried as Decision.Service - not the
// service an operation belongs to, but the filter list_capabilities itself
// takes (see Decision's own doc comment, internal/usecase/planner.go).
func decisionFromListCapabilities(args map[string]any) usecase.Decision {
	return usecase.Decision{
		Kind:    usecase.DecisionListCapabilities,
		Service: stringArg(args, "service"),
	}
}

// decisionFromCall builds a DecisionCall for any tool name other than
// ask_user or list_capabilities: name is the operation id (usecase.ToolsFor
// names a tool after nothing else), so the service it belongs to must be
// looked up in the catalogue.
func (p *Planner) decisionFromCall(name string, args map[string]any) (usecase.Decision, error) {
	service, ok := resolveService(p.catalog, name)
	if !ok {
		return usecase.Decision{}, fmt.Errorf("%w: %q", ErrUnknownOperation, name)
	}

	return usecase.Decision{Kind: usecase.DecisionCall, Service: service, OperationID: name, Args: args}, nil
}

// resolveService looks operationID up in catalog and returns the service
// that owns it.
//
// A tool call carries only the operation id - usecase.ToolsFor never
// qualifies a tool's Name with its service, because the tool-calling wire
// format has no separate field for it - so when two services in the
// catalogue declare the very same operation id, the call alone cannot say
// which one was meant. That is a real ambiguity, not a bug in this
// function: it is resolved by taking the first match in catalog.Endpoints
// order, which is deterministic (the catalogue is built by iterating
// configured services in a fixed order) but not necessarily correct. This
// is judged an acceptable default rather than an error, because avoiding it
// is a naming discipline on the services themselves - keep operation ids
// unique across the catalogue, which every operation id in this product so
// far already is (ListInventoryItems vs. ListAttendanceRecords, not
// List) - not something this planner can fix after the fact. See
// DECISIONS.md.
func resolveService(catalog domain.Catalog, operationID string) (string, bool) {
	for i := range catalog.Endpoints {
		if catalog.Endpoints[i].OperationID == operationID {
			return catalog.Endpoints[i].Service, true
		}
	}

	return "", false
}

// stringArg reads a string argument, defaulting to "" when absent or of
// the wrong type - a model that gets ask_user's schema wrong should not
// crash the planner, just produce a Decision Orchestrator.ask will reject
// with ErrEndpointNotFound or ErrUnknownParam.
func stringArg(args map[string]any, name string) string {
	s, ok := args[name].(string)
	if !ok {
		return ""
	}

	return s
}

// optionsArg reads ask_user's "options" argument into []domain.Option. See
// decisionFromAskUser for why the result is carried but never trusted
// downstream.
func optionsArg(args map[string]any, name string) []domain.Option {
	raw, ok := args[name].([]any)
	if !ok {
		return nil
	}

	options := make([]domain.Option, 0, len(raw))

	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		options = append(options, domain.Option{Value: stringArg(m, "value"), Label: stringArg(m, "label")})
	}

	return options
}

// buildMessages renders one system message and one user message: the
// question, followed - when answers is non-empty - by every answer the
// person has already given to a previous ask_user question, so a
// resubmitted query actually uses the chosen value instead of asking
// again. There is no assistant/tool turn here even though answers were
// produced by a previous ask_user call: each /api/plan request is one
// stateless chat completion (D8, docs/specs/orchestration.md - one LLM
// call per request), not a continuation of a stored conversation, so the
// only way an answer reaches the model at all is folded into this turn's
// own text.
func buildMessages(query string, answers []usecase.Answer) []chat.Message {
	content := query

	if len(answers) > 0 {
		var b strings.Builder

		b.WriteString(query)
		b.WriteString("\n\nこれまでに確認した値:\n")

		for _, a := range answers {
			fmt.Fprintf(&b, "- %s = %s\n", a.Param, a.Value)
		}

		b.WriteString("\n上記の値をそのまま使って、対応する操作を呼び出してください。")

		content = b.String()
	}

	return []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: content},
	}
}

// shapeTools converts every usecase.Tool into a chat.ToolDefinition.
func shapeTools(tools []usecase.Tool) []chat.ToolDefinition {
	out := make([]chat.ToolDefinition, len(tools))
	for i, t := range tools {
		out[i] = shapeTool(&t)
	}

	return out
}

// shapeTool converts one usecase.Tool into the wire shape: usecase.ToolsFor
// deliberately emits a plain JSON Schema, unshaped for any one transport's
// strict-mode rules (internal/usecase/tools.go), because Task 11's JSON
// planner needs it that way too - so shaping it for tool calling's strict
// mode is this adapter's job, done here rather than in chat.Client, which
// knows nothing about usecase.Tool.
//
// OpenAI's strict function calling requires additionalProperties: false on
// every object and every declared property to be listed as required - an
// optional filter such as ListInventoryItems's "status" is not. Rather
// than fabricate a required list the model was never told about (which
// would make ListInventoryItems's own catalogue-declared "optional"
// dishonest to the model), a tool that has any optional top-level property
// is sent with strict: false. additionalProperties: false is added
// regardless of strict, top-level and on every nested object schema
// (a Response/RequestBody in this catalogue never nests an object inside
// another - see the openapi.yaml of each dummy service - so a shallow walk
// covers every case that exists today): it costs nothing on a backend that
// ignores it (llama.cpp/llama-swap, verified by hand, DECISIONS.md) and it
// is the harmless half of strict mode - it forbids an invented extra
// argument, not a real optional one - on a backend that enforces it. See
// DECISIONS.md for the writeup of this tradeoff.
func shapeTool(t *usecase.Tool) chat.ToolDefinition {
	schema := shapeSchema(t.InputSchema)
	strict := t.Strict && everyPropertyRequired(schema)

	return chat.ToolDefinition{
		Type: "function",
		Function: chat.FunctionDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
			Strict:      &strict,
		},
	}
}

// shapeSchema deep-copies a JSON Schema map, adding "additionalProperties":
// false to every object along the way (its own top level, its "items", and
// every one of its "properties").
func shapeSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	out := make(map[string]any, len(schema)+1)

	for k, v := range schema {
		switch k {
		case "properties":
			out[k] = shapeProperties(v)
		case "items":
			out[k] = shapeNested(v)
		default:
			out[k] = v
		}
	}

	if out["type"] == domain.SchemaTypeObject {
		out["additionalProperties"] = false
	}

	return out
}

// shapeProperties applies shapeSchema to every value of a "properties" map.
func shapeProperties(v any) any {
	props, ok := v.(map[string]any)
	if !ok {
		return v
	}

	out := make(map[string]any, len(props))
	for name, prop := range props {
		out[name] = shapeNested(prop)
	}

	return out
}

// shapeNested applies shapeSchema to one nested schema value (a
// "properties" entry or an "items" schema), tolerating a value that is not
// itself a schema map - usecase.ToolsFor never produces one, but this
// function has no way to prove that of an arbitrary map[string]any.
func shapeNested(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}

	return shapeSchema(m)
}

// everyPropertyRequired reports whether a (already-shaped) top-level
// object schema's "required" list names every one of its "properties" -
// the condition strict mode needs to be honest, per shapeTool's doc
// comment. A schema with no properties at all is vacuously true.
func everyPropertyRequired(schema map[string]any) bool {
	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return true
	}

	required, ok := schema["required"].([]string)
	if !ok {
		required = nil
	}

	requiredSet := make(map[string]struct{}, len(required))
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}

	for name := range props {
		if _, ok := requiredSet[name]; !ok {
			return false
		}
	}

	return true
}
