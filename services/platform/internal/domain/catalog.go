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
	// Description is the operation's own OpenAPI description - distinct
	// from Summary, and the one place a real service's `also` terms live
	// (docs/specs/shortlisting.md, H1; e2e/narrowing/lexical.ts's own
	// combinedTextOf joins it beside summary, display name and service
	// display name). Empty when the contract declares none.
	Description string
	Parameters  []Parameter
	RequestBody *Schema
	Response    *Schema
	// UIHint is the operation's x-ui-hint.component override. Empty when
	// the spec gives none, in which case Render decides from the response
	// schema.
	UIHint Component
	// ChartHint is the operation's x-ui-hint.chart declaration: the axes
	// to draw a result of this endpoint's response with. nil when the
	// contract declares none. Its presence alone is enough for Render and
	// RenderResult to choose ComponentChart (docs/specs/dashboard.md,
	// P2): a contract that names a chart's axes is declaring that this is
	// how the result draws, not offering axes for some other component to
	// use.
	ChartHint *Chart
	// DisplayName is the operation's x-ui-hint.displayName: a name for a
	// person to read, as distinct from Summary, which is the model-facing
	// tool description (usecase.ToolsFor) and stays in whatever language
	// the contract's author wrote it in (DECISIONS.md, 2026-09-13). Empty
	// when the contract declares none - see DisplayNameOr.
	DisplayName string
	// ServiceDisplayName is the endpoint's service's own
	// info.x-ui-hint.displayName: a name for a person to read, one level
	// up from DisplayName and read from the same extension one level up
	// in the contract (the info object, not an operation) - the
	// identifier Service names is what ORCHESTRA_SERVICES, source.service
	// and a Permission row all carry, and stays exactly that (DECISIONS.md,
	// 2026-09-13). Empty when the contract declares none - see
	// ServiceDisplayNameOr.
	ServiceDisplayName string
	// Examples is the operation's x-orchestra-examples: things a person
	// might type when they want it, written by the service owner into the
	// contract (docs/specs/describing.md, section 3). nil when the
	// contract declares none. Inert for now - nothing downstream of
	// /api/catalog reads it yet.
	Examples []string
}

// DisplayNameOr returns e.DisplayName when the contract declares one,
// otherwise fallback. Each caller passes whatever it already showed a
// person before this field existed - /api/catalog falls back to Summary,
// list_capabilities' table falls back to OperationID - so a contract that
// says nothing about a display name changes nothing anyone already sees
// (DECISIONS.md, 2026-09-13).
func (e *Endpoint) DisplayNameOr(fallback string) string {
	if e.DisplayName != "" {
		return e.DisplayName
	}

	return fallback
}

// ServiceDisplayNameOr returns e.ServiceDisplayName when the service's
// contract declares one, otherwise fallback - the same shape as
// DisplayNameOr, one level up (DECISIONS.md, 2026-09-13).
func (e *Endpoint) ServiceDisplayNameOr(fallback string) string {
	if e.ServiceDisplayName != "" {
		return e.ServiceDisplayName
	}

	return fallback
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
