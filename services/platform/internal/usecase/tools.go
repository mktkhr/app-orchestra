package usecase

import (
	"maps"
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
	keyEnumLabels  = "enumLabels"
)

// paramService names the "service" argument shared by ask_user and
// list_capabilities (each with a different meaning - see their own doc
// comments) and the "service" column of a list_capabilities result. Named
// once, here, so the literal doesn't drift and to satisfy goconst
// (harness/quality/go/golangci.yml, min-occurrences: 3).
const paramService = "service"

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
const askUserDescription = "Call this ONLY when the question does not tell you which value to use for " +
	"a parameter that declares a fixed set of allowed values (an enum), and you need the person to pick " +
	"one from that set. Do NOT call this for a free-text parameter (for example a name) that has no " +
	"declared set of values - there is nothing to pick from, so this tool cannot help; leave that " +
	"parameter out of your call instead."

// askUserServiceDescription and askUserOperationIDDescription explain why
// ask_user must name the operation it is standing in for: a parameter name
// such as "status" or "type" is not unique across a catalogue of many
// services, so naming the parameter alone is not enough to know which
// endpoint's enum the person is being asked about.
const (
	askUserServiceDescription = "The service that owns the operation you were about to call before " +
		"the parameter's value stopped you - the same service name that tool would have used."
	askUserOperationIDDescription = "The operation id of the call you were about to make before the " +
		"parameter's value stopped you - i.e. the tool you would have called instead of ask_user."
)

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
				paramService: map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: askUserServiceDescription,
				},
				"operationId": map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: askUserOperationIDDescription,
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
			keyRequired: []string{"question", paramService, "operationId", "param", "options"},
		},
		Strict: true,
	}
}

// listCapabilitiesDescription explains, to the model, when to reach for
// list_capabilities instead of guessing an answer from memory: any
// question about what the platform (or one named service) can do at all,
// as opposed to a question that names a concrete thing to look up or
// change.
const listCapabilitiesDescription = "Call this when the question asks what operations are " +
	"available - in general (\"何ができるの？\") or for one named service (\"在庫について、どういう" +
	"操作ができる？\") - rather than asking to actually look something up or change something. " +
	"Returns the catalogue's own list of operations, so it never risks naming a capability that " +
	"does not exist."

// listCapabilitiesServiceDescription explains service: it is deliberately
// not an enum (docs/specs/orchestration.md; the harness's x-enum-labels
// lint would force one), because the set of services grows over time and
// this tool is not derived from any one service's spec the way a real enum
// parameter is.
const listCapabilitiesServiceDescription = "Optional. Restrict the answer to one service, named " +
	"exactly as it is used elsewhere in this catalogue (for example \"inventory\" or " +
	"\"attendance\"). Omit it to list every service's operations."

// ListCapabilitiesTool is one further tool, always present, not derived
// from any service's spec (docs/specs/orchestration.md, section 8): it
// lets the model answer "what can this do?" from the catalogue itself
// instead of either refusing the question or inventing an answer.
//
// It is a function rather than a package-level value for the same
// gochecknoglobals reason AskUserTool is (see its own doc comment).
func ListCapabilitiesTool() Tool {
	return Tool{
		Name:        "list_capabilities",
		Description: listCapabilitiesDescription,
		InputSchema: map[string]any{
			keyType: domain.SchemaTypeObject,
			keyProperties: map[string]any{
				paramService: map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: listCapabilitiesServiceDescription,
				},
			},
		},
		Strict: true,
	}
}

// ToolsFor converts a catalogue into the tool definitions a planner offers
// the model: one per endpoint the catalogue carries, plus AskUserTool and
// ListCapabilitiesTool.
//
// Every endpoint here is already one the operator marked
// x-orchestra-expose: true (internal/adapter/specsource/http.parseSpec) —
// that is the only filter the catalogue applies, and it is applied once,
// before ToolsFor and Catalog.Find both read the same c.Endpoints, so the
// tool list offered to the model and the operations /api/invoke will
// answer can never disagree (docs/specs/orchestration.md, section 8;
// DECISIONS.md). ToolsFor itself no longer excludes an endpoint by the
// shape of its schemas: an exposed endpoint with neither a Response nor a
// RequestBody schema — one Render (internal/domain/rendering.go) could
// never draw a component for — is a mistake in the spec, not a case to
// silently drop. harness/guard/exposed-ops.sh catches it before it reaches
// here.
// builtinToolCount is how many tools ToolsFor adds beyond the catalogue's
// own endpoints (AskUserTool, ListCapabilitiesTool) - named so the
// capacity hint below isn't a bare "magic number" (mnd,
// harness/quality/go/golangci.yml).
const builtinToolCount = 2

func ToolsFor(c domain.Catalog) []Tool {
	tools := make([]Tool, 0, len(c.Endpoints)+builtinToolCount)

	for i := range c.Endpoints {
		e := &c.Endpoints[i]

		tools = append(tools, Tool{
			Name:        e.OperationID,
			Description: e.Summary,
			InputSchema: inputSchemaFor(e),
			Strict:      true,
		})
	}

	tools = append(tools, AskUserTool(), ListCapabilitiesTool())

	return tools
}

// inputSchemaFor builds the JSON Schema object describing an endpoint's
// arguments: its parameters, plus the request body's own properties merged
// in at the top level (a create call's body fields are just more
// arguments, from the model's point of view).
func inputSchemaFor(e *domain.Endpoint) map[string]any {
	properties := map[string]any{}

	required := make([]string, 0, len(e.Parameters))
	safe := e.IsSafe()

	for i := range e.Parameters {
		// Indexed rather than ranged: Parameter is 136 bytes, and
		// gocritic's rangeValCopy (part of the fixed harness policy)
		// rejects copying it per iteration.
		p := &e.Parameters[i]

		if EnumParamGetsSyntheticAll(safe, p) {
			withAll := WithSyntheticAll(&p.Schema)
			properties[p.Name] = schemaToJSONSchema(&withAll)
			required = append(required, p.Name)

			continue
		}

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
// Task 5 — that remains the only form the model itself reads. The same
// labels are also carried as a structured `enumLabels` map (value ->
// label), which is what a UI consumes instead of parsing the description
// string apart (see DECISIONS.md).
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
		m[keyEnumLabels] = enumLabelsMap(s.Enum, s.EnumLabels)
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

// EnumParamGetsSyntheticAll reports whether p should be offered to the
// model as required, with domain.EnumAllValue added to its enum (D15,
// docs/specs/orchestration.md section 8a): a safe endpoint's optional enum
// parameter, and no other case. Exported so both planners that read a
// catalogue's schemas agree on when the synthetic value applies instead of
// each deciding separately: internal/adapter/planner/toolcall's tools come
// straight from ToolsFor/inputSchemaFor, but
// internal/adapter/planner/jsonmode renders its own catalogue text
// directly from domain.Endpoint and must reach the same answer.
//
// p is a pointer, not the value: domain.Parameter is large enough that
// gocritic's hugeParam check (harness/quality/go/golangci.yml) rejects
// copying it per call.
func EnumParamGetsSyntheticAll(safe bool, p *domain.Parameter) bool {
	return safe && !p.Required && len(p.Schema.Enum) > 0
}

// syntheticAllInstruction tells the model when the synthetic
// domain.EnumAllValue (D15) is and is not the right answer: it competes
// with ask_user (calling a list tool is easier than calling a different
// tool), and the measurement in DECISIONS.md's 2026-09-12 entry found the
// unqualified value made that competition worse, not better - this is the
// one lever tried before reverting D15.
const syntheticAllInstruction = "__all__ は利用者が全件を求めたときだけ選ぶこと。" +
	"利用者が挙げた語がどの値にも当てはまらないときは __all__ を選ばず、ask_user で聞き返すこと。"

// WithSyntheticAll returns a copy of s with domain.EnumAllValue appended to
// Enum, labelled domain.EnumAllLabel in EnumLabels, and
// syntheticAllInstruction appended to Description (joined the same way
// describe joins a description to its enum labels: a single space, and
// only when the description is not empty already) so the model reads one
// coherent sentence-then-labels string rather than finding the instruction
// buried mid-way. It never mutates s: s is the domain.Schema held by
// domain.Catalog, which Catalog.Find and the ask_user path both read, and
// neither may ever see the synthetic value (D15).
func WithSyntheticAll(s *domain.Schema) domain.Schema {
	enum := make([]string, len(s.Enum)+1)
	copy(enum, s.Enum)
	enum[len(s.Enum)] = domain.EnumAllValue

	labels := make(map[string]string, len(s.EnumLabels)+1)
	maps.Copy(labels, s.EnumLabels)
	labels[domain.EnumAllValue] = domain.EnumAllLabel

	out := *s
	out.Enum = enum
	out.EnumLabels = labels

	if out.Description == "" {
		out.Description = syntheticAllInstruction
	} else {
		out.Description = out.Description + " " + syntheticAllInstruction
	}

	return out
}

// enumLabelsMap builds the structured value->label map schemaToJSONSchema
// carries alongside enum, for a UI to look a value's label up by key
// instead of parsing describe's string apart. Same defensive choice as
// enumLabels: a value with no entry in labels (a spec violation the
// harness's own x-enum-labels lint would already have caught upstream)
// gets an empty label rather than being dropped.
func enumLabelsMap(enum []string, labels map[string]string) map[string]string {
	out := make(map[string]string, len(enum))
	for _, value := range enum {
		out[value] = labels[value]
	}

	return out
}
