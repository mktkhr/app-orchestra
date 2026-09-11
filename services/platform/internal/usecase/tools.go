package usecase

import (
	"sort"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// JSON Schema key names, shared by every schema built below. Named once so
// the repeated literal doesn't drift and to satisfy goconst.
const (
	keyType        = "type"
	keyProperties  = "properties"
	keyDescription = "description"
	keyRequired    = "required"
	keyEnum        = "enum"
	keyItems       = "items"
	keyTitle       = "title"
)

// Tool is one function the model can call: one catalogue endpoint, or the
// fixed ask_user escape hatch. It is deliberately spec-agnostic (a plain
// JSON Schema map, not an SDK type) so both the tool-calling planner and the
// text-based JSON planner (Task 11) can render it their own way from the
// same value.
type Tool struct {
	// Name is the operation id (or "ask_user").
	Name string
	// Description is the operation summary (or ask_user's fixed text).
	Description string
	// InputSchema is a JSON Schema object: {"type": "object",
	// "properties": {...}, "required": [...]}.
	InputSchema map[string]any
	// Strict is always true: see D10 in docs/specs/orchestration.md.
	Strict bool
}

// askUserDescription explains, to the model, when to reach for ask_user
// instead of one of the catalogue's own tools.
const askUserDescription = "Call this when the question does not tell you which value to use for an " +
	"enum parameter. It hands the choice back to the person instead of guessing."

// AskUserTool is one further tool, always present, not derived from any
// service's spec (docs/specs/orchestration.md, section 8): it lets the
// model say "I cannot tell which value you mean" and hand the choice back
// to the person (D11).
//
// It is a function rather than a package-level value (gochecknoglobals is
// part of the fixed lint policy, harness/quality/go/golangci.yml) so callers
// each get their own InputSchema map instead of sharing — and risking a
// mutation of — one global.
func AskUserTool() Tool {
	return Tool{
		Name:        "ask_user",
		Description: askUserDescription,
		InputSchema: map[string]any{
			keyType: domain.SchemaTypeObject,
			keyProperties: map[string]any{
				"question": map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: "The question to show the person, in Japanese.",
				},
				"param": map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: "The name of the parameter the answer will fill in.",
				},
				"options": map[string]any{
					keyType: domain.SchemaTypeArray,
					keyItems: map[string]any{
						keyType: domain.SchemaTypeObject,
						keyProperties: map[string]any{
							"value": map[string]any{keyType: domain.SchemaTypeString},
							"label": map[string]any{keyType: domain.SchemaTypeString},
						},
						keyRequired: []string{"value", "label"},
					},
					keyDescription: "The candidate values, each with its Japanese label, for the person to pick from.",
				},
			},
			keyRequired: []string{"question", "param", "options"},
		},
		Strict: true,
	}
}

// ToolsFor converts a catalogue into the tool definitions a planner offers
// the model: one per endpoint that can produce something a component can
// render, plus AskUserTool.
//
// An endpoint with neither a Response nor a RequestBody schema is excluded
// rather than turned into a tool. This is a property of the endpoint, not a
// name check against operation ids such as "GetSpec": an endpoint with no
// response schema and no request body is, by construction, one Render (see
// internal/domain/rendering.go) can never draw a component for, because
// Render's rules all key off one of those two schemas. The one such
// endpoint every generated service carries is `GET /openapi.yaml`, which
// returns its own contract as raw YAML rather than a domain.Schema — asking
// the model to choose it would be offering a tool whose result the browser
// could never show.
func ToolsFor(c domain.Catalog) []Tool {
	tools := make([]Tool, 0, len(c.Endpoints)+1)

	for i := range c.Endpoints {
		e := &c.Endpoints[i]
		if e.Response == nil && e.RequestBody == nil {
			continue
		}

		tools = append(tools, Tool{
			Name:        e.OperationID,
			Description: e.Summary,
			InputSchema: inputSchemaFor(e),
			Strict:      true,
		})
	}

	tools = append(tools, AskUserTool())

	return tools
}

// inputSchemaFor builds the JSON Schema object describing an endpoint's
// arguments: its parameters, plus the request body's own properties merged
// in at the top level (a create call's body fields are just more
// arguments, from the model's point of view).
func inputSchemaFor(e *domain.Endpoint) map[string]any {
	properties := map[string]any{}

	required := make([]string, 0, len(e.Parameters))

	for i := range e.Parameters {
		// Indexed rather than ranged: Parameter is 136 bytes, and
		// gocritic's rangeValCopy (part of the fixed harness policy)
		// rejects copying it per iteration.
		p := &e.Parameters[i]

		properties[p.Name] = schemaToJSONSchema(&p.Schema)

		if p.Required {
			required = append(required, p.Name)
		}
	}

	if e.RequestBody != nil {
		required = append(required, mergeRequestBody(properties, e.RequestBody)...)
	}

	schema := map[string]any{
		keyType:       domain.SchemaTypeObject,
		keyProperties: properties,
	}

	if len(required) > 0 {
		schema[keyRequired] = dedupeSorted(required)
	}

	return schema
}

// mergeRequestBody adds a request body's fields to an endpoint's input
// schema, and returns the names that must additionally be marked required.
// An object body contributes its properties directly, since they are
// exactly the arguments a create/update call takes, along with its own
// required property names; anything else (a bare scalar or array body) is
// carried as a single "body" property so no information is silently
// dropped, and "body" itself is required, since the body as a whole is.
func mergeRequestBody(properties map[string]any, body *domain.Schema) []string {
	if body.Type == domain.SchemaTypeObject {
		for name, prop := range body.Properties {
			properties[name] = schemaToJSONSchema(&prop)
		}

		return body.Required
	}

	properties["body"] = schemaToJSONSchema(body)

	return []string{"body"}
}

// dedupeSorted sorts names and removes duplicates, so the input schema's
// required list is deterministic regardless of how many sources (an
// endpoint's own parameters, a merged request body) contributed to it.
func dedupeSorted(names []string) []string {
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)

	out := sorted[:0:0]

	for i, name := range sorted {
		if i == 0 || name != sorted[i-1] {
			out = append(out, name)
		}
	}

	return out
}

// schemaToJSONSchema converts a domain.Schema into a JSON Schema map. An
// enum's Japanese labels (x-enum-labels) are appended to the property's
// description as "value=label / value=label", per docs/plans/orchestration.md
// Task 5 — this is the only place those labels reach the model.
func schemaToJSONSchema(s *domain.Schema) map[string]any {
	m := map[string]any{}

	if s.Type != "" {
		m[keyType] = s.Type
	}

	if s.Title != "" {
		m[keyTitle] = s.Title
	}

	if len(s.Enum) > 0 {
		enum := make([]string, len(s.Enum))
		copy(enum, s.Enum)
		m[keyEnum] = enum
	}

	if description := describe(s); description != "" {
		m[keyDescription] = description
	}

	if s.Type == domain.SchemaTypeArray && s.Items != nil {
		m[keyItems] = schemaToJSONSchema(s.Items)
	}

	if s.Type == domain.SchemaTypeObject && len(s.Properties) > 0 {
		props := make(map[string]any, len(s.Properties))
		for name, prop := range s.Properties {
			props[name] = schemaToJSONSchema(&prop)
		}

		m[keyProperties] = props
	}

	if s.Type == domain.SchemaTypeObject && len(s.Required) > 0 {
		required := make([]string, len(s.Required))
		copy(required, s.Required)
		m[keyRequired] = required
	}

	return m
}

// describe builds a property's description: its own Description, followed
// by its enum labels when it has any.
func describe(s *domain.Schema) string {
	if len(s.Enum) == 0 {
		return s.Description
	}

	labels := enumLabels(s.Enum, s.EnumLabels)
	if s.Description == "" {
		return labels
	}

	return s.Description + " " + labels
}

// enumLabels renders an enum's values and Japanese labels as
// "allocated=引当済 / staged=出荷準備完了", in enum order. A value with no
// label (a spec violation the harness's own x-enum-labels lint would
// already have caught upstream) is rendered with an empty label rather than
// dropped, so a gap is visible instead of silently disappearing.
func enumLabels(enum []string, labels map[string]string) string {
	parts := make([]string, len(enum))
	for i, value := range enum {
		parts[i] = value + "=" + labels[value]
	}

	return strings.Join(parts, " / ")
}
