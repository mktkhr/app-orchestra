package domain

// ComponentChart is a fifth value Component can hold, alongside the four
// Render and RenderResult choose between. It never comes back from either
// function: a chart is something a panel's own View asks for, or a
// contract's x-ui-hint declares (docs/specs/dashboard.md, P2) - never
// something the response schema alone implies, the way a table or a
// detail is. A person or a contract sets Component to it directly.
const ComponentChart Component = "chart"

// Render answers what to draw for an endpoint the platform is choosing
// between: it has not been called, and may never be. No I/O, no model; the
// component is decided entirely from the spec, per
// docs/specs/orchestration.md section 6.
//
// The rules, in order:
//  1. Endpoint.UIHint, when set, wins outright.
//  2. A request body means the operation is unsafe: the model's arguments
//     become a form instead of being invoked.
//  3. Otherwise the response schema decides it, as RenderResult describes.
//
// The parameter is a pointer, not the value shown in the plan, because
// Endpoint is 136 bytes: golangci-lint's gocritic hugeParam check (part of
// the fixed harness policy, see harness/quality/go/golangci.yml) rejects
// passing it by value.
func Render(e *Endpoint) Component {
	if e.UIHint != "" {
		return e.UIHint
	}

	if e.RequestBody != nil {
		return ComponentForm
	}

	return RenderResult(e)
}

// RenderResult answers what to draw for a result the endpoint has already
// produced. It is the same rule as Render minus the form: a request body
// says something about making the call, and the call has been made, so it
// says nothing about the answer that came back.
//
// This is the rule /api/invoke renders with, and the rule Render falls
// through to once it knows the call is safe.
//
// The rules, in order:
//  1. Endpoint.UIHint, when set, wins outright.
//  2. An object array — either the response itself, or the single
//     array-valued property of a wrapper object such as
//     {items: [...], total: n} — means a table.
//  3. A single object means a detail view.
//
// A response that fits none of these (nil, or a bare scalar) renders as no
// component at all: RenderResult returns "".
func RenderResult(e *Endpoint) Component {
	if e.UIHint != "" {
		return e.UIHint
	}

	if isObjectArray(e.Response) {
		return ComponentTable
	}

	if e.Response != nil && e.Response.Type == SchemaTypeObject {
		return ComponentDetail
	}

	return ""
}

// FieldsSchema returns the schema whose Properties describe the columns a
// rendered result carries: for a table, the row schema (the array's own
// Items when the response is a bare array, or the wrapper's sole array
// property's Items for an envelope such as {items: [...], total: n}); for
// a detail, the response schema itself. It answers the same "what does this
// endpoint's response look like" question RenderResult does, and by the
// same rules, so a caller building a per-field schema (docs/specs/orchestration.md
// section 6, `fields`) never has to re-derive which property of a wrapper
// object holds the rows - that judgment is made exactly once, here.
//
// It returns nil when RenderResult would render neither a table nor a
// detail (an ask/form path, or a response with no component at all), so a
// caller knows not to produce a fields output at all rather than an empty
// one.
func FieldsSchema(e *Endpoint) *Schema {
	switch RenderResult(e) {
	case ComponentTable:
		return tableRowSchema(e.Response)
	case ComponentDetail:
		return e.Response
	default:
		return nil
	}
}

// tableRowSchema returns the schema of one row of a table response: s
// itself when it already is an array (Items), or the sole array property's
// Items when s is a wrapper object. Mirrors isObjectArray's own judgment of
// where the rows live, so the two can never disagree about which property
// holds them.
func tableRowSchema(s *Schema) *Schema {
	if s == nil {
		return nil
	}

	if s.Type == SchemaTypeArray {
		return s.Items
	}

	if s.Type == SchemaTypeObject {
		if arr := soleArrayProperty(s); arr != nil {
			return arr.Items
		}
	}

	return nil
}

// isObjectArray reports whether s is an array of objects, either directly
// or as the single array-valued property of a wrapper object (a paginated
// envelope such as {items: [...], total: n}).
func isObjectArray(s *Schema) bool {
	if s == nil {
		return false
	}

	if s.Type == SchemaTypeArray {
		return s.Items != nil && s.Items.Type == SchemaTypeObject
	}

	if s.Type == SchemaTypeObject {
		if arr := soleArrayProperty(s); arr != nil {
			return arr.Items != nil && arr.Items.Type == SchemaTypeObject
		}
	}

	return false
}

// soleArrayProperty returns the schema's single array-typed property, when
// it has exactly one. A wrapper object with more than one array property, or
// none, is not a wrapper for the purposes of Render.
func soleArrayProperty(s *Schema) *Schema {
	var found *Schema

	for name := range s.Properties {
		prop := s.Properties[name]
		if prop.Type != SchemaTypeArray {
			continue
		}

		if found != nil {
			return nil // more than one array property: not a wrapper
		}

		found = &prop
	}

	return found
}
