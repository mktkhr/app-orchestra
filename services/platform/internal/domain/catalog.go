// Package domain holds the platform's core types and pure rules: the
// catalogue of endpoints gathered from the configured services, and the
// rule that decides which component renders a result. Nothing in this
// package performs I/O.
package domain

// HTTP method names used by Endpoint.Method. Defined locally, rather than
// taken from net/http, because the domain layer may not import it (see
// harness/quality/go/golangci.yml, depguard): net/http is a transport
// concern, and QUERY, which these methods also need, has no stdlib
// constant anyway.
const (
	MethodGet   = "GET"
	MethodHead  = "HEAD"
	MethodQuery = "QUERY"
)

// Component names the frontend widget that renders an endpoint's result.
type Component string

// The four components the rendering rule can choose.
const (
	ComponentTable  Component = "table"
	ComponentDetail Component = "detail"
	ComponentForm   Component = "form"
	ComponentChoice Component = "choice"
)

// The JSON Schema type names a Schema.Type can hold.
const (
	SchemaTypeObject  = "object"
	SchemaTypeArray   = "array"
	SchemaTypeString  = "string"
	SchemaTypeInteger = "integer"
	SchemaTypeBoolean = "boolean"
)

// Schema is a minimal, spec-agnostic description of a JSON value: enough of
// OpenAPI's schema object for the rendering rule and the catalogue to work
// from, and no more.
type Schema struct {
	// Type is one of the SchemaType constants above.
	Type string
	// Items describes the element type when Type is "array".
	Items *Schema
	// Properties describes the fields when Type is "object".
	Properties map[string]Schema
	// Required lists the names of the properties that must be present
	// when Type is "object".
	Required []string
	// Enum lists the allowed values for a string schema.
	Enum []string
	// EnumLabels maps each Enum value to its Japanese label.
	EnumLabels map[string]string
	// Title is the schema's display title, if any.
	Title string
	// Description is the schema's free-text description, if any.
	Description string
}

// Parameter is one request parameter of an Endpoint.
type Parameter struct {
	Name     string
	In       string // "query", "path"
	Required bool
	Schema   Schema
}

// Endpoint is one operation of one service, as reflected from its OpenAPI
// contract.
type Endpoint struct {
	Service     string
	OperationID string
	Method      string // GET, POST, PUT, PATCH, DELETE, HEAD, QUERY
	Path        string
	Summary     string
	Parameters  []Parameter
	RequestBody *Schema
	Response    *Schema
	// UIHint is the operation's x-ui-hint.component override. Empty when
	// the spec gives none, in which case Render decides from the response
	// schema.
	UIHint Component
}

// IsSafe reports whether the endpoint's method never mutates state (GET,
// HEAD, QUERY). QUERY is included per the spec
// (docs/specs/orchestration.md, section 13): a read-only search that
// happens to be sent as QUERY is still safe.
//
// The receiver is a pointer, not the value shown in the plan, because
// Endpoint is 136 bytes: golangci-lint's gocritic hugeParam check (part of
// the fixed harness policy, see harness/quality/go/golangci.yml) rejects
// passing it by value.
func (e *Endpoint) IsSafe() bool {
	switch e.Method {
	case MethodGet, MethodHead, MethodQuery:
		return true
	default:
		return false
	}
}

// EnumAllValue and EnumAllLabel are the synthetic enum value D15
// (docs/specs/orchestration.md, section 8a) adds to an optional enum
// parameter on a safe endpoint, so the model is offered it as required:
// leaving the parameter out stops being a legal answer, and asking for
// every row becomes something the model must say rather than something it
// falls into by failing to match a word. The value never reaches a
// service or a result's provenance - the orchestrator strips it before an
// argument is validated or a call is made (see usecase.stripSyntheticAll) -
// and domain.Catalog itself never carries it: it is added only to the
// JSON Schema a planner offers the model, never to a Schema.Enum here.
const (
	EnumAllValue = "__all__"
	EnumAllLabel = "すべて"
)

// Option is one candidate value a person can pick from, with its Japanese
// label. Used by a planner's ask_user decision (D11,
// docs/specs/orchestration.md) to hand an ambiguous enum value back to the
// user instead of guessing.
type Option struct {
	Value string
	Label string
}

// Catalog is the set of endpoints gathered from every configured service.
type Catalog struct {
	Endpoints []Endpoint
}

// For returns the subset of c whose endpoints permissions names: an
// endpoint is kept only when some permission's (Service, OperationID)
// matches it exactly (docs/specs/auth.md, section 4, A3). No permissions
// at all yields an empty catalogue, not the whole one - a person with no
// rows may call nothing, and an admin never reaches this method at all
// (docs/specs/auth.md, section 4): the caller decides that, not For.
//
// This is the filter docs/specs/auth.md section 5 draws as the single
// point both the tool list and /api/invoke read from - it is applied
// once, by the caller, and its result is handed to both ToolsFor and
// Find so the two can never disagree.
//
// Pure: For touches nothing but its own arguments, as every function in
// this package must (harness/quality/go/golangci.yml, depguard forbids
// this package from importing anything beyond the standard library).
func (c Catalog) For(permissions []Permission) Catalog {
	allowed := make(map[Permission]struct{}, len(permissions))
	for _, p := range permissions {
		allowed[p] = struct{}{}
	}

	endpoints := make([]Endpoint, 0, len(c.Endpoints))

	for i := range c.Endpoints {
		key := Permission{Service: c.Endpoints[i].Service, OperationID: c.Endpoints[i].OperationID}
		if _, ok := allowed[key]; ok {
			endpoints = append(endpoints, c.Endpoints[i])
		}
	}

	return Catalog{Endpoints: endpoints}
}

// Find looks up the endpoint with the given service and operation id.
func (c Catalog) Find(service, operationID string) (Endpoint, bool) {
	for i := range c.Endpoints {
		if c.Endpoints[i].Service == service && c.Endpoints[i].OperationID == operationID {
			return c.Endpoints[i], true
		}
	}
	return Endpoint{}, false
}
