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
	keyEnumLabels  = "enumLabels"
)

// paramService names the "service" argument shared by ask_user,
// list_capabilities and propose_panel (each with a different meaning - see
// their own doc comments) and the "service" column of a list_capabilities
// result. Named once, here, so the literal doesn't drift and to satisfy
// goconst (harness/quality/go/golangci.yml, min-occurrences: 3).
const paramService = "service"

// paramOperationID names the "operationId" argument shared by ask_user and
// propose_panel, for the same goconst reason paramService is named once.
const paramOperationID = "operationId"

// paramValue names the "value" property ask_user's own "options" items and
// propose_panel's "chart" argument both declare, for the same goconst
// reason paramService is named once.
const paramValue = "value"

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
	// Examples is the operation's own x-orchestra-examples
	// (domain.Endpoint.Examples), carried here as plain data - the
	// usecase layer renders no prompt text itself (docs/specs/wording.md,
	// section 3). nil for a tool that is not a catalogue operation
	// (ask_user, list_capabilities, propose_panel) or whose endpoint
	// declared none. A planner adapter decides what, if anything, to do
	// with it: internal/adapter/planner/toolcall's wording.CatalogueTool
	// is the only reader today, and under wording.Default() (v1) it
	// ignores this field entirely, so its presence changes nothing on the
	// wire until a different wording is selected.
	Examples []string
}

// askUserDescription explains, to the model, when to reach for ask_user
// instead of one of the catalogue's own tools.
const askUserDescription = "Call this ONLY when the question does not tell you which value to use for " +
	"a parameter that declares a fixed set of allowed values (an enum), and you need the person to pick " +
	"one from that set. Do NOT call this for a free-text parameter (for example a name) that has no " +
	"declared set of values - there is nothing to pick from, so this tool cannot help; leave that " +
	"parameter out of your call instead."

// askUserOperationIDDescription explains why ask_user must name the
// operation it is standing in for: a parameter name such as "status" or
// "type" is not unique across a catalogue of many services, so naming the
// parameter alone is not enough to know which endpoint's enum the person
// is being asked about. There is deliberately no matching "service"
// argument: with a five-service catalogue the model has no reliable way to
// name a service it never called (measured 2026-09-15,
// docs/specs/shortlisting.md - fabricated service names such as
// "approval" or "salesBundle" 500'd as ErrEndpointNotFound), so the
// planner resolves the service from operationId itself, the same way it
// already does for a real tool call (resolveService).
const askUserOperationIDDescription = "The operation id of the call you were about to make before the " +
	"parameter's value stopped you - i.e. the tool you would have called instead of ask_user. Its service " +
	"is looked up automatically; do not name it yourself."

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
				paramOperationID: map[string]any{
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
							paramValue: map[string]any{keyType: domain.SchemaTypeString},
							"label":    map[string]any{keyType: domain.SchemaTypeString},
						},
						keyRequired: []string{paramValue, "label"},
					},
					keyDescription: "The candidate values, each with its Japanese label, for the person to pick from.",
				},
			},
			keyRequired: []string{"question", paramOperationID, "param", "options"},
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

// proposePanelDescription explains, to the model, when to reach for
// propose_panel instead of calling a safe operation directly: a question
// that asks for something to be put on the workspace's screen, as opposed
// to a question that asks to look something up. It never does anything
// itself (docs/specs/proposing.md, N1) - the platform turns the call into
// an offer a person still has to place.
const proposePanelDescription = "Call this when the question asks to put something on the workspace's " +
	"screen - a panel, a chart, a table - rather than asking a question you should just answer. Name the " +
	"operation and arguments the panel's data should come from, exactly as you would for that operation's " +
	"own tool. component, chart, transform and title are all optional: leave any of them out and the " +
	"platform fills it in from the same rule it would have drawn the answer with. Never call this for a " +
	"question that only asks to look something up - call that operation's own tool instead."

// proposePanelArgsDescription, proposePanelComponentDescription,
// proposePanelChartDescription, proposePanelTransformDescription and
// proposePanelTitleDescription are propose_panel's own per-parameter
// descriptions - kept as named constants for the same reason
// askUserServiceDescription is (readability of ProposePanelTool below).
const (
	proposePanelArgsDescription = "The arguments the named operation should be called with, exactly as " +
		"that operation's own tool declares them."
	proposePanelComponentDescription = "Optional. The widget to draw the panel with. Leave it out to let " +
		"the platform choose the same way it would have for a plain answer."
	proposePanelChartDescription = "Optional. The chart's axes, when component is \"chart\" and the " +
		"operation's own contract does not already declare them."
	proposePanelTransformDescription = "Optional. How to group and reduce the rows before drawing them - " +
		"leave it out when the operation's own response needs no grouping."
	proposePanelTitleDescription = "Optional. The panel's title. Leave it out to use the operation's own " +
		"display name."
)

// proposePanelChartSchema and proposePanelTransformSchema build
// propose_panel's "chart" and "transform" argument schemas: the same
// shape the contract's View schema declares (docs/specs/dashboard.md,
// section 3), reused here rather than invented a second time
// (docs/plans/proposing.md, Task 0's own constraint).
func proposePanelChartSchema() map[string]any {
	return map[string]any{
		keyType:        domain.SchemaTypeObject,
		keyDescription: proposePanelChartDescription,
		keyProperties: map[string]any{
			"category": map[string]any{keyType: domain.SchemaTypeString, keyDescription: "The field named as the chart's category axis."},
			paramValue: map[string]any{keyType: domain.SchemaTypeString, keyDescription: "The field named as the chart's value axis."},
			"kind":     map[string]any{keyType: domain.SchemaTypeString, keyEnum: []string{"bar", "line", "pie"}},
		},
		keyRequired: []string{"category", paramValue, "kind"},
	}
}

func proposePanelTransformSchema() map[string]any {
	return map[string]any{
		keyType:        domain.SchemaTypeObject,
		keyDescription: proposePanelTransformDescription,
		keyProperties: map[string]any{
			"groupBy":   map[string]any{keyType: domain.SchemaTypeString, keyDescription: "The field whose distinct values become rows."},
			"aggregate": map[string]any{keyType: domain.SchemaTypeString, keyEnum: []string{"count", "sum", "avg"}},
			"field":     map[string]any{keyType: domain.SchemaTypeString, keyDescription: "The field to aggregate. Absent for \"count\"."},
		},
		keyRequired: []string{"groupBy", "aggregate"},
	}
}

// ProposePanelToolName mirrors ProposePanelTool's own Name: named once so
// a caller that only needs to compare against it (Orchestrator.Plan's
// ErrToolNotOffered check, docs/specs/offering.md O5) does not have to
// build the whole Tool value just to read one field off it.
const ProposePanelToolName = "propose_panel"

// ProposePanelTool is one further tool, always present, not derived from
// any service's spec: it lets the model answer with a panel it composed
// rather than doing anything (docs/specs/proposing.md, N1, section 3). It
// is a function rather than a package-level value for the same
// gochecknoglobals reason AskUserTool is (see its own doc comment).
func ProposePanelTool() Tool {
	return Tool{
		Name:        ProposePanelToolName,
		Description: proposePanelDescription,
		InputSchema: map[string]any{
			keyType: domain.SchemaTypeObject,
			keyProperties: map[string]any{
				paramService: map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: "The service the operation belongs to, exactly as that operation's own tool names it.",
				},
				paramOperationID: map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: "The operation id the panel's data should come from.",
				},
				"args": map[string]any{
					keyType:        domain.SchemaTypeObject,
					keyDescription: proposePanelArgsDescription,
				},
				"component": map[string]any{
					keyType:        domain.SchemaTypeString,
					keyEnum:        []string{"table", "detail", "form", "choice", "chart"},
					keyDescription: proposePanelComponentDescription,
				},
				"chart":     proposePanelChartSchema(),
				"transform": proposePanelTransformSchema(),
				"title": map[string]any{
					keyType:        domain.SchemaTypeString,
					keyDescription: proposePanelTitleDescription,
				},
			},
			keyRequired: []string{paramService, paramOperationID, "args"},
		},
		Strict: true,
	}
}

// PlanContext is what one /api/plan request carries that a BuiltinTool's
// own condition can read (O2, docs/specs/offering.md) - built once, in
// Orchestrator.Plan, from what the request itself said, never from the
// conversation's content (section 5's second exclusion): a condition that
// read what the model said would be a tool the model could talk its way
// into.
//
// It grows when a condition needs something new; today it carries only
// whether the question came from a workspace (O3, O4).
type PlanContext struct {
	// WorkspaceID is the workspace the question was asked from, or "" for
	// a question asked from the chat screen, which has no workspace to put
	// a panel on (docs/specs/proposing.md, N4).
	WorkspaceID string
}

// BuiltinTool is one further tool ToolsFor may add beyond the catalogue's
// own endpoints: its definition, plus the condition - if any - that
// decides whether one request is offered it at all (O1,
// docs/specs/offering.md). Applies is nil for a tool that applies to every
// request, which is what ask_user and list_capabilities do (AC-O-103) -
// "always present" is the absence of a condition, not a case the mechanism
// has to special-case.
type BuiltinTool struct {
	Tool    Tool
	Applies func(PlanContext) bool
}

// appliesFromWorkspace is O3: propose_panel applies only when the question
// was asked from a workspace - what ConversationPanel already knows and
// already uses to decide whether a proposal can be drawn at all
// (docs/specs/proposing.md, N4; web/src/widgets/conversation/ui/ConversationPanel.tsx).
func appliesFromWorkspace(ctx PlanContext) bool {
	return ctx.WorkspaceID != ""
}

// builtinTools lists every tool ToolsFor adds beyond the catalogue's own
// endpoints, in the order it appends them. Order matters here: it is what
// keeps ToolsFor's output stable (AC-O-105) regardless of which condition
// holds for a given request.
func builtinTools() []BuiltinTool {
	return []BuiltinTool{
		{Tool: AskUserTool()},
		{Tool: ListCapabilitiesTool()},
		{Tool: ProposePanelTool(), Applies: appliesFromWorkspace},
	}
}

// ToolsFor converts a catalogue into the tool definitions a planner offers
// the model: one per endpoint the catalogue carries, plus every
// builtinTools entry whose condition planCtx satisfies (O2) - a tool whose
// condition is not met is not in the list the model sees, not disabled and
// not described as unavailable (section 5's third exclusion).
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
// builtinToolCount is how many tools builtinTools declares - named so the
// capacity hint below isn't a bare "magic number" (mnd,
// harness/quality/go/golangci.yml). It is a capacity hint, not a
// guarantee: fewer may actually be appended, when planCtx fails a
// condition.
const builtinToolCount = 3

func ToolsFor(c domain.Catalog, planCtx PlanContext) []Tool {
	tools := make([]Tool, 0, len(c.Endpoints)+builtinToolCount)

	for i := range c.Endpoints {
		tools = append(tools, toolFor(&c.Endpoints[i]))
	}

	for _, bt := range builtinTools() {
		if bt.Applies == nil || bt.Applies(planCtx) {
			tools = append(tools, bt.Tool)
		}
	}

	return tools
}

// toolFor builds the one Tool a single catalogue endpoint converts to:
// ToolsFor's own per-endpoint step, factored out so Orchestrator.planPreferred
// (orchestrator.go) can offer a lone operation's tool without also pulling in
// ToolsFor's built-ins - see planPreferred's own doc comment for why.
func toolFor(e *domain.Endpoint) Tool {
	return Tool{
		Name:        e.OperationID,
		Description: e.Summary,
		InputSchema: inputSchemaFor(e),
		Strict:      true,
		Examples:    e.Examples,
	}
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
