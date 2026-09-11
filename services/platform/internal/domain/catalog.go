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

// Find looks up the endpoint with the given service and operation id.
func (c Catalog) Find(service, operationID string) (Endpoint, bool) {
	for i := range c.Endpoints {
		if c.Endpoints[i].Service == service && c.Endpoints[i].OperationID == operationID {
			return c.Endpoints[i], true
		}
	}
	return Endpoint{}, false
}
