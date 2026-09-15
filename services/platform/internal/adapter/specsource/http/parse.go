package http

import (
	"errors"
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
// at /api/invoke (see extExpose below); x-orchestra-examples carries things
// a person might type when they want an operation, written by the service
// owner (docs/specs/describing.md, section 3; see examplesHint below).
const (
	extEnumLabels = "x-enum-labels"
	extUIHint     = "x-ui-hint"
	extExpose     = "x-orchestra-expose"
	extExamples   = "x-orchestra-examples"
)

// jsonMediaType is the only content type the catalogue looks for in a
// request body or response: every operation these contracts expose speaks
// JSON. GET /openapi.yaml speaks YAML instead, which is one reason (not
// having a JSON schema to render) it is never marked x-orchestra-expose,
// and so never reaches parseSpec's endpoints slice at all.
const jsonMediaType = "application/json"

// errMalformedChartHint marks an x-ui-hint.chart that cannot be parsed:
// not an object, missing one of its three required fields, or a kind
// outside bar/line/pie. Unlike x-ui-hint.component (see uiHint) this fails
// the fetch rather than degrading to "no chart" - a hint nobody can see
// failing is a hint that silently stops working, and a chart's axes are
// never guessable the way "fall back to the response schema" is for a
// component.
var errMalformedChartHint = errors.New("malformed x-ui-hint.chart")

// errMalformedExamples marks an x-orchestra-examples that is present but
// is not an array of strings. Like errMalformedChartHint (see its own
// comment) and unlike x-ui-hint.component's leniency, this fails the fetch
// rather than dropping the field silently: an example nobody can see
// failing to parse is an example that silently stops existing, and there
// is no fallback to degrade to the way component falls back to the
// response schema.
var errMalformedExamples = errors.New("malformed x-orchestra-examples")

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

	var svcDisplayName string
	if doc.Info != nil {
		svcDisplayName = serviceDisplayName(doc.Info.Extensions)
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

			component, chart, displayName, err := uiHint(op.Extensions)
			if err != nil {
				return nil, fmt.Errorf("%s %s %s: %w", service, method, path, err)
			}

			examples, err := examplesHint(op.Extensions)
			if err != nil {
				return nil, fmt.Errorf("%s %s %s: %w", service, method, path, err)
			}

			endpoints = append(endpoints, domain.Endpoint{
				Service:            service,
				OperationID:        op.OperationID,
				Method:             method,
				Path:               path,
				Summary:            op.Summary,
				Description:        op.Description,
				Parameters:         convertParameters(op.Parameters),
				RequestBody:        convertRequestBody(op.RequestBody),
				Response:           convertResponse(op.Responses),
				UIHint:             component,
				ChartHint:          chart,
				DisplayName:        displayName,
				ServiceDisplayName: svcDisplayName,
				Examples:           examples,
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

// uiHint reads x-ui-hint off an operation's extensions: component, an
// override the rendering rule already accepts (docs/plans/dashboard.md
// Task 3 asks for its sibling, not a rewrite of it), chart, a contract's
// own declaration of a result's axes (see parseChartHint), and
// displayName, a name for a person to read (docs/specs/orchestration.md
// D7; DECISIONS.md, 2026-09-13) - distinct from Summary, which is the
// model-facing tool description and is never translated for display.
//
// component and displayName stay lenient exactly as component always has:
// absent, a non-object x-ui-hint, or a non-string value all mean "no
// override"/"no display name", not an error - both fall back to something
// already shown (the response schema for component, Summary or the
// operation id for displayName; see domain.Endpoint.DisplayNameOr), so
// there is nothing to fail loudly about. chart is not given the same
// leniency; see errMalformedChartHint.
func uiHint(extensions map[string]any) (domain.Component, *domain.Chart, string, error) {
	m := extractUIHint(extensions)
	if m == nil {
		return "", nil, "", nil
	}

	var component string
	if s, ok := m["component"].(string); ok {
		component = s
	}

	var chart *domain.Chart

	if raw, hasChart := m["chart"]; hasChart {
		c, err := parseChartHint(raw)
		if err != nil {
			return "", nil, "", err
		}

		chart = c
	}

	return domain.Component(component), chart, displayNameOf(m), nil
}

// serviceDisplayName reads x-ui-hint.displayName off a service's own
// info object - the same extension uiHint reads off an operation, one
// level up (docs/specs/orchestration.md D16; DECISIONS.md, 2026-09-13).
// It shares uiHint's leniency: absent, a non-object x-ui-hint, or a
// non-string displayName all mean "no display name", never an error -
// every domain.Endpoint of the service simply falls back to Service (see
// domain.Endpoint.ServiceDisplayNameOr), exactly as an operation without
// one falls back to Summary or its operation id.
func serviceDisplayName(extensions map[string]any) string {
	return displayNameOf(extractUIHint(extensions))
}

// extractUIHint reads the x-ui-hint object off any extensions map - an
// operation's or, one level up, a service's own info object. Absent, or a
// non-object value, both yield nil: "no hints at all", the one case
// uiHint and serviceDisplayName each need to tell apart from "hints, but
// no displayName among them".
func extractUIHint(extensions map[string]any) map[string]any {
	raw, ok := extensions[extUIHint]
	if !ok {
		return nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	return m
}

// displayNameOf reads displayName off an already-extracted x-ui-hint
// object (see extractUIHint). A nil m or a non-string value both yield "",
// meaning "no display name" to every caller.
func displayNameOf(m map[string]any) string {
	if s, ok := m["displayName"].(string); ok {
		return s
	}

	return ""
}

// parseChartHint converts x-ui-hint.chart into a domain.Chart. hasChart is
// false when the operation declares no chart at all, which is not an
// error - a contract that says nothing about its axes is simply not
// declaring one, and parseChartHint is not even called (see uiHint).
// Anything present but malformed - not an object, missing category or
// value, or a kind outside bar/line/pie - is errMalformedChartHint: see
// its own comment for why this differs from component's leniency.
func parseChartHint(raw any) (*domain.Chart, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: not an object", errMalformedChartHint)
	}

	category, ok := m["category"].(string)
	if !ok || category == "" {
		return nil, fmt.Errorf("%w: category is required", errMalformedChartHint)
	}

	value, ok := m["value"].(string)
	if !ok || value == "" {
		return nil, fmt.Errorf("%w: value is required", errMalformedChartHint)
	}

	kindRaw, ok := m["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: kind is required", errMalformedChartHint)
	}

	kind := domain.ChartKind(kindRaw)

	switch kind {
	case domain.ChartKindBar, domain.ChartKindLine, domain.ChartKindPie:
	default:
		return nil, fmt.Errorf("%w: kind %q is not bar, line or pie", errMalformedChartHint, kindRaw)
	}

	return &domain.Chart{Category: category, Value: value, Kind: kind}, nil
}

// examplesHint reads x-orchestra-examples off an operation's extensions:
// things a person might type when they want it (docs/specs/describing.md,
// section 3). Absent yields nil, nil - no examples, not an error. Present
// as anything other than an array of strings yields errMalformedExamples;
// see its own comment for why this is not treated as leniently as
// x-ui-hint.component is.
func examplesHint(extensions map[string]any) ([]string, error) {
	raw, ok := extensions[extExamples]
	if !ok {
		return nil, nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: not an array", errMalformedExamples)
	}

	examples := make([]string, 0, len(items))

	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%w: element %v is not a string", errMalformedExamples, item)
		}

		examples = append(examples, s)
	}

	return examples, nil
}
