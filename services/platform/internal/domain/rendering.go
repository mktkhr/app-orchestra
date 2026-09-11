package domain

// Render is a pure function of an endpoint's response schema. No I/O, no
// model: the component a result is drawn with is decided entirely from the
// spec, per docs/specs/orchestration.md section 6.
//
// The rules, in order:
//  1. Endpoint.UIHint, when set, wins outright.
//  2. A request body means the operation is unsafe: the model's arguments
//     become a form instead of being invoked.
//  3. An object array — either the response itself, or the single
//     array-valued property of a wrapper object such as
//     {items: [...], total: n} — means a table.
//  4. A single object means a detail view.
//
// A response that fits none of these (nil, or a bare scalar) renders as no
// component at all: Render returns "".
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

	if isObjectArray(e.Response) {
		return ComponentTable
	}

	if e.Response != nil && e.Response.Type == SchemaTypeObject {
		return ComponentDetail
	}

	return ""
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
