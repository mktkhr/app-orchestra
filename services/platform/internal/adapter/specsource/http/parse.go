package http

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// The vendor extensions the catalogue reads out of a service's contract.
// x-enum-labels carries the Japanese label for each enum value (the only
// route by which the model learns it); x-ui-hint.component overrides the
// component the rendering rule would otherwise pick; x-orchestra-expose
// marks an operation as one the platform may show to the model and answer
// at /api/invoke (see extExpose below).
const (
	extEnumLabels = "x-enum-labels"
	extUIHint     = "x-ui-hint"
	extExpose     = "x-orchestra-expose"
)

// jsonMediaType is the only content type the catalogue looks for in a
// request body or response: every operation these contracts expose speaks
// JSON. GET /openapi.yaml speaks YAML instead, which is one reason (not
// having a JSON schema to render) it is never marked x-orchestra-expose,
// and so never reaches parseSpec's endpoints slice at all.
const jsonMediaType = "application/json"

// parseSpec parses one service's OpenAPI document and converts every
// operation into a domain.Endpoint. The loader resolves the document's
// internal $refs (the contracts here never reference another file), so
// convertSchema always sees a resolved *openapi3.Schema.
func parseSpec(service string, data []byte) ([]domain.Endpoint, error) {
	doc, err := openapi3.NewLoader().LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("parsing openapi document: %w", err)
	}

	if doc.Paths == nil {
		return nil, nil
	}

	paths := doc.Paths.Keys()
	sort.Strings(paths)

	var endpoints []domain.Endpoint

	for _, path := range paths {
		item := doc.Paths.Value(path)

		for _, method := range sortedMethods(item.Operations()) {
			op := item.Operations()[method]

			if !isExposed(op.Extensions) {
				continue
			}

			endpoints = append(endpoints, domain.Endpoint{
				Service:     service,
				OperationID: op.OperationID,
				Method:      method,
				Path:        path,
				Summary:     op.Summary,
				Parameters:  convertParameters(op.Parameters),
				RequestBody: convertRequestBody(op.RequestBody),
				Response:    convertResponse(op.Responses),
				UIHint:      uiHint(op.Extensions),
			})
		}
	}

	return endpoints, nil
}

// sortedMethods returns the operation's HTTP methods in a fixed order, so
// the catalogue does not depend on Go's randomised map iteration.
func sortedMethods(operations map[string]*openapi3.Operation) []string {
	methods := make([]string, 0, len(operations))
	for method := range operations {
		methods = append(methods, method)
	}

	sort.Strings(methods)

	return methods
}

// convertParameters converts every OpenAPI parameter into a
// domain.Parameter.
func convertParameters(params openapi3.Parameters) []domain.Parameter {
	if len(params) == 0 {
		return nil
	}

	converted := make([]domain.Parameter, 0, len(params))

	for _, ref := range params {
		if ref == nil || ref.Value == nil {
			continue
		}

		p := ref.Value

		var schema domain.Schema
		if s := convertSchema(p.Schema); s != nil {
			schema = *s
		}

		converted = append(converted, domain.Parameter{
			Name:     p.Name,
			In:       p.In,
			Required: p.Required,
			Schema:   schema,
		})
	}

	return converted
}

// convertRequestBody converts the request body's JSON schema, when the
// operation has one.
func convertRequestBody(ref *openapi3.RequestBodyRef) *domain.Schema {
	if ref == nil || ref.Value == nil {
		return nil
	}

	mt := ref.Value.Content.Get(jsonMediaType)
	if mt == nil {
		return nil
	}

	return convertSchema(mt.Schema)
}

// convertResponse picks the first successful (2xx) JSON response, in
// ascending status order, and converts its schema. A response with no JSON
// content (GET /openapi.yaml's application/yaml body, for instance) yields
// no schema at all, which Render already treats as no component.
func convertResponse(responses *openapi3.Responses) *domain.Schema {
	if responses == nil {
		return nil
	}

	statuses := responses.Keys()
	sort.Strings(statuses)

	for _, status := range statuses {
		if !strings.HasPrefix(status, "2") {
			continue
		}

		ref := responses.Value(status)
		if ref == nil || ref.Value == nil {
			continue
		}

		mt := ref.Value.Content.Get(jsonMediaType)
		if mt == nil {
			continue
		}

		return convertSchema(mt.Schema)
	}

	return nil
}

// convertSchema converts a resolved OpenAPI schema into a domain.Schema,
// recursively for array items and object properties. It returns nil when
// ref has no resolved value.
func convertSchema(ref *openapi3.SchemaRef) *domain.Schema {
	if ref == nil || ref.Value == nil {
		return nil
	}

	s := ref.Value

	out := &domain.Schema{
		Title:       s.Title,
		Description: s.Description,
	}

	if s.Type != nil && !s.Type.IsEmpty() {
		out.Type = s.Type.Slice()[0]
	}

	if len(s.Enum) > 0 {
		out.Enum = convertEnum(s.Enum)
	}

	if labels, ok := s.Extensions[extEnumLabels]; ok {
		out.EnumLabels = toStringMap(labels)
	}

	if s.Items != nil {
		out.Items = convertSchema(s.Items)
	}

	if len(s.Properties) > 0 {
		out.Properties = convertProperties(s.Properties)
	}

	if len(s.Required) > 0 {
		out.Required = append([]string(nil), s.Required...)
	}

	return out
}

// convertEnum renders every enum value (decoded from YAML/JSON as `any`)
// as its string form.
func convertEnum(values []any) []string {
	enum := make([]string, 0, len(values))
	for _, v := range values {
		enum = append(enum, fmt.Sprint(v))
	}

	return enum
}

// convertProperties converts an object schema's properties.
func convertProperties(properties openapi3.Schemas) map[string]domain.Schema {
	out := make(map[string]domain.Schema, len(properties))

	for name, ref := range properties {
		if converted := convertSchema(ref); converted != nil {
			out[name] = *converted
		}
	}

	return out
}

// toStringMap converts an x-enum-labels extension value (a
// map[string]any once decoded from YAML/JSON) into a map[string]string.
// A malformed or absent extension yields nil rather than an error: a
// missing label is a spec-quality problem, not a reason to fail the fetch.
func toStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}

	out := make(map[string]string, len(m))
	for key, val := range m {
		out[key] = fmt.Sprint(val)
	}

	return out
}

// isExposed reads x-orchestra-expose off an operation's extensions. Only
// `true` counts as exposed: absent, `false`, or any non-boolean value (a
// typo such as a quoted `"true"`) all mean the operation stays out of the
// catalogue. The default is closed rather than open on purpose — a service
// the operator has not yet reviewed is invisible until someone marks an
// operation, not exposed until someone remembers to hide it.
func isExposed(extensions map[string]any) bool {
	raw, ok := extensions[extExpose]
	if !ok {
		return false
	}

	exposed, ok := raw.(bool)

	return ok && exposed
}

// uiHint reads x-ui-hint.component off an operation's extensions.
func uiHint(extensions map[string]any) domain.Component {
	raw, ok := extensions[extUIHint]
	if !ok {
		return ""
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return ""
	}

	component, ok := m["component"].(string)
	if !ok {
		return ""
	}

	return domain.Component(component)
}
