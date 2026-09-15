package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// catalogWithEnumParameter mirrors the shape of the inventory service's
// real catalogue (see the plan for Task 5), assembled by hand so this test
// does not depend on a running service.
func catalogWithEnumParameter() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ListInventoryItems",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムの一覧を返す",
			Parameters: []domain.Parameter{
				{
					Name:     "status",
					In:       "query",
					Required: false,
					Schema: domain.Schema{
						Type: domain.SchemaTypeString,
						Enum: []string{"allocated", "staged", "quarantined", "consigned"},
						EnumLabels: map[string]string{
							"allocated":   "引当済",
							"staged":      "出荷準備完了",
							"quarantined": "検品保留",
							"consigned":   "預託在庫",
						},
					},
				},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
		},
		{
			Service:     "inventory",
			OperationID: "CreateInventoryItem",
			Method:      "POST",
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムを作成する",
			RequestBody: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"name": {Type: domain.SchemaTypeString},
				},
			},
		},
		{
			Service:     "inventory",
			OperationID: "GetInventoryItem",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/items/{id}",
			Summary:     "在庫アイテムを取得する",
			Response:    &domain.Schema{Type: domain.SchemaTypeObject},
		},
	}}
}

func TestToolsForBuildsOneToolPerCatalogueEndpointPlusAskUser(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	// 3 catalogue endpoints (ListInventoryItems, CreateInventoryItem,
	// GetInventoryItem) + ask_user + list_capabilities + propose_panel.
	// The catalogue never carries an unexposed operation such as GetSpec
	// in the first place - that filter runs once, in
	// specsource/http.parseSpec, before ToolsFor ever sees c.Endpoints.
	require.Len(t, tools, 6)

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	assert.Contains(t, names, "ListInventoryItems")
	assert.Contains(t, names, "CreateInventoryItem")
	assert.Contains(t, names, "GetInventoryItem")

	askUserCount := 0
	for _, name := range names {
		if name == "ask_user" {
			askUserCount++
		}
	}
	assert.Equal(t, 1, askUserCount, "ask_user must be present exactly once")
}

func TestToolsForIncludesListCapabilitiesExactlyOnce(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	// 3 catalogue endpoints + ask_user + list_capabilities + propose_panel.
	require.Len(t, tools, 6)

	count := 0
	for _, tool := range tools {
		if tool.Name == "list_capabilities" {
			count++
		}
	}
	assert.Equal(t, 1, count, "list_capabilities must be present exactly once")
}

func TestListCapabilitiesToolHasAnOptionalServiceParameterThatIsNotAnEnum(t *testing.T) {
	tool := usecase.ListCapabilitiesTool()

	assert.Equal(t, "list_capabilities", tool.Name)
	assert.NotEmpty(t, tool.Description)

	properties, ok := tool.InputSchema["properties"].(map[string]any)
	require.True(t, ok, "input schema must carry a properties map")

	serviceProp, ok := properties["service"].(map[string]any)
	require.True(t, ok, "input schema must carry the service property")
	assert.Equal(t, "string", serviceProp["type"])
	_, hasEnum := serviceProp["enum"]
	assert.False(t, hasEnum, "service must not be an enum: service names are added dynamically")

	_, hasRequired := tool.InputSchema["required"]
	assert.False(t, hasRequired, "service is the tool's only property and it is optional, so there is no required list at all")
}

// TestToolsForIncludesProposePanelExactlyOnce is AC-N-106's precondition:
// propose_panel must reach the tool list ToolsFor builds, alongside
// ask_user and list_capabilities, on every catalogue.
func TestToolsForIncludesProposePanelExactlyOnce(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	count := 0
	for _, tool := range tools {
		if tool.Name == "propose_panel" {
			count++
		}
	}
	assert.Equal(t, 1, count, "propose_panel must be present exactly once")
}

// TestProposePanelToolNamesExactlyTheParametersSection3Declares is
// docs/specs/proposing.md, section 3: propose_panel(service, operationId,
// args, component?, chart?, transform?, title?) - service, operationId and
// args required, the panel's own view and title optional.
func TestProposePanelToolNamesExactlyTheParametersSection3Declares(t *testing.T) {
	tool := usecase.ProposePanelTool()

	assert.Equal(t, "propose_panel", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Strict)

	properties, ok := tool.InputSchema["properties"].(map[string]any)
	require.True(t, ok, "input schema must carry a properties map")

	for _, name := range []string{"service", "operationId", "args", "component", "chart", "transform", "title"} {
		assert.Contains(t, properties, name)
	}

	required, ok := tool.InputSchema["required"].([]string)
	require.True(t, ok, "input schema must carry a required list")
	assert.ElementsMatch(t, []string{"service", "operationId", "args"}, required)

	componentProp, ok := properties["component"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"table", "detail", "form", "choice", "chart"}, componentProp["enum"])

	chartProp, ok := properties["chart"].(map[string]any)
	require.True(t, ok)
	chartProperties, ok := chartProp["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, chartProperties, "category")
	assert.Contains(t, chartProperties, "value")
	assert.Contains(t, chartProperties, "kind")

	transformProp, ok := properties["transform"].(map[string]any)
	require.True(t, ok)
	transformProperties, ok := transformProp["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, transformProperties, "groupBy")
	assert.Contains(t, transformProperties, "aggregate")
	assert.Contains(t, transformProperties, "field")
}

func TestToolsForDoesNotExcludeAnEndpointWithNoResponseAndNoRequestBody(t *testing.T) {
	// Whether an endpoint can be rendered is no longer ToolsFor's business:
	// only x-orchestra-expose (enforced upstream, in specsource/http and
	// harness/guard/exposed-ops.sh) decides whether an endpoint reaches the
	// catalogue at all. A catalogue endpoint with neither schema still
	// becomes a tool here — a spec bug for the guard to catch, not
	// something ToolsFor silently drops.
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "PingInventory",
			Method:      domain.MethodGet,
			Path:        "/api/inventory/ping",
			Summary:     "returns nothing a component can render",
			// Response and RequestBody both nil.
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 4)
	names := []string{tools[0].Name, tools[1].Name, tools[2].Name}
	assert.Contains(t, names, "PingInventory")
	assert.Contains(t, names, "ask_user")
	assert.Contains(t, names, "list_capabilities")
}

func TestToolsForSetsStrictTrue(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	for _, tool := range tools {
		assert.True(t, tool.Strict, "tool %q must be strict", tool.Name)
	}
}

func TestToolsForUsesOperationSummaryAsDescription(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	found := false
	for _, tool := range tools {
		if tool.Name == "ListInventoryItems" {
			found = true
			assert.Equal(t, "在庫アイテムの一覧を返す", tool.Description)
		}
	}
	assert.True(t, found)
}

// TestToolsForCopiesEndpointExamplesOntoTheTool is
// docs/plans/wording.md, Task 1, Step 3: ToolsFor carries a catalogue
// endpoint's own x-orchestra-examples (domain.Endpoint.Examples) onto
// usecase.Tool.Examples as plain data - the usecase layer renders no
// prompt text itself (docs/specs/wording.md, section 3); only a planner
// adapter's selected wording decides what, if anything, to do with it.
func TestToolsForCopiesEndpointExamplesOntoTheTool(t *testing.T) {
	c := catalogWithEnumParameter()
	c.Endpoints[0].Examples = []string{"在庫を見せて", "在庫の状況を教えて"}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	found := false

	for _, tool := range tools {
		if tool.Name == "ListInventoryItems" {
			found = true

			assert.Equal(t, []string{"在庫を見せて", "在庫の状況を教えて"}, tool.Examples)
		}
	}

	assert.True(t, found)
}

// TestToolsForWithNoEndpointExamplesLeavesToolExamplesNil is the other
// half of TestToolsForCopiesEndpointExamplesOntoTheTool's contract: an
// endpoint with no x-orchestra-examples (catalogWithEnumParameter's own,
// unmodified) produces a nil Examples, not an empty-but-non-nil slice a
// caller would have to tell apart from "one example, empty string".
func TestToolsForWithNoEndpointExamplesLeavesToolExamplesNil(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	found := false

	for _, tool := range tools {
		if tool.Name == "ListInventoryItems" {
			found = true

			assert.Nil(t, tool.Examples)
		}
	}

	assert.True(t, found)
}

func TestToolsForEnumParameterCarriesEnumAndJapaneseLabelsInDescription(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	var listTool usecase.Tool
	for _, tool := range tools {
		if tool.Name == "ListInventoryItems" {
			listTool = tool
		}
	}
	require.NotEmpty(t, listTool.Name)

	properties, ok := listTool.InputSchema["properties"].(map[string]any)
	require.True(t, ok, "input schema must carry a properties map")

	statusProp, ok := properties["status"].(map[string]any)
	require.True(t, ok, "input schema must carry the status property")

	assert.Equal(t, []string{"allocated", "staged", "quarantined", "consigned"}, statusProp["enum"])
	if required, isSlice := listTool.InputSchema["required"].([]string); isSlice {
		assert.NotContains(t, required, "status", "status is an optional enum parameter and must stay optional")
	}

	description, ok := statusProp["description"].(string)
	require.True(t, ok, "the status property must carry a description")
	assert.Contains(t, description, "allocated=引当済")
	assert.Contains(t, description, "staged=出荷準備完了")
	assert.Contains(t, description, "quarantined=検品保留")
	assert.Contains(t, description, "consigned=預託在庫")
}

func TestToolsForEnumParameterAlsoCarriesStructuredEnumLabels(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	var listTool usecase.Tool
	for _, tool := range tools {
		if tool.Name == "ListInventoryItems" {
			listTool = tool
		}
	}
	require.NotEmpty(t, listTool.Name)

	properties, ok := listTool.InputSchema["properties"].(map[string]any)
	require.True(t, ok, "input schema must carry a properties map")

	statusProp, ok := properties["status"].(map[string]any)
	require.True(t, ok, "input schema must carry the status property")

	enumLabels, ok := statusProp["enumLabels"].(map[string]string)
	require.True(t, ok, "the status property must carry a structured enumLabels map")

	assert.Equal(t, map[string]string{
		"allocated":   "引当済",
		"staged":      "出荷準備完了",
		"quarantined": "検品保留",
		"consigned":   "預託在庫",
	}, enumLabels)

	// The description form (for the model) must still be present alongside
	// the structured one (for the UI) — this is an addition, not a
	// replacement.
	description, ok := statusProp["description"].(string)
	require.True(t, ok, "the status property must still carry a description")
	assert.Contains(t, description, "allocated=引当済")
}

func TestToolsForCreateEndpointMergesRequestBodyIntoInputSchema(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	var createTool usecase.Tool
	for _, tool := range tools {
		if tool.Name == "CreateInventoryItem" {
			createTool = tool
		}
	}
	require.NotEmpty(t, createTool.Name)

	properties, ok := createTool.InputSchema["properties"].(map[string]any)
	require.True(t, ok)
	_, ok = properties["name"]
	assert.True(t, ok, "the request body's fields must reach the input schema")
}

func TestToolsForMarksRequiredParametersInOrder(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "attendance",
			OperationID: "ListAttendanceRecords",
			Method:      domain.MethodGet,
			Path:        "/api/attendance/records",
			Summary:     "勤怠記録の一覧を返す",
			Parameters: []domain.Parameter{
				{Name: "zeta", In: "query", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
				{Name: "alpha", In: "query", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
				{Name: "optional", In: "query", Required: false, Schema: domain.Schema{Type: domain.SchemaTypeString}},
			},
			Response: &domain.Schema{Type: domain.SchemaTypeArray, Items: &domain.Schema{Type: domain.SchemaTypeObject}},
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	var listTool usecase.Tool
	for _, tool := range tools {
		if tool.Name == "ListAttendanceRecords" {
			listTool = tool
		}
	}
	require.NotEmpty(t, listTool.Name)
	assert.Equal(t, []string{"alpha", "zeta"}, listTool.InputSchema["required"])
}

func TestToolsForNonObjectRequestBodyBecomesBodyProperty(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "ReplaceInventoryCsv",
			Method:      "PUT",
			Path:        "/api/inventory/csv",
			Summary:     "在庫データをCSVで置き換える",
			RequestBody: &domain.Schema{Type: domain.SchemaTypeString},
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 4)
	properties, ok := tools[0].InputSchema["properties"].(map[string]any)
	require.True(t, ok)

	body, ok := properties["body"].(map[string]any)
	require.True(t, ok, "a non-object request body must be carried as a single body property")
	assert.Equal(t, domain.SchemaTypeString, body["type"])
	assert.Equal(t, []string{"body"}, tools[0].InputSchema["required"],
		"a scalar/array body is itself required, since it carries the whole payload")
}

func TestToolsForObjectRequestBodyMergesRequiredFieldsIntoTopLevel(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "CreateInventoryItem",
			Method:      "POST",
			Path:        "/api/inventory/items",
			Summary:     "在庫アイテムを作成する",
			RequestBody: &domain.Schema{
				Type:     domain.SchemaTypeObject,
				Required: []string{"name", "status", "quantity"},
				Properties: map[string]domain.Schema{
					"name":     {Type: domain.SchemaTypeString},
					"status":   {Type: domain.SchemaTypeString},
					"quantity": {Type: domain.SchemaTypeInteger},
					"note":     {Type: domain.SchemaTypeString},
				},
			},
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 4)
	assert.Equal(t, []string{"name", "quantity", "status"}, tools[0].InputSchema["required"],
		"the request body's required fields must reach the top-level required list, deduped and sorted")
}

func TestToolsForMergesParameterAndRequestBodyRequiredNamesDeduped(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "UpdateInventoryItem",
			Method:      "PUT",
			Path:        "/api/inventory/items/{id}",
			Summary:     "在庫アイテムを更新する",
			Parameters: []domain.Parameter{
				{Name: "id", In: "path", Required: true, Schema: domain.Schema{Type: domain.SchemaTypeString}},
			},
			RequestBody: &domain.Schema{
				Type:     domain.SchemaTypeObject,
				Required: []string{"id", "status"},
				Properties: map[string]domain.Schema{
					"id":     {Type: domain.SchemaTypeString},
					"status": {Type: domain.SchemaTypeString},
				},
			},
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 4)
	assert.Equal(t, []string{"id", "status"}, tools[0].InputSchema["required"],
		"a name required by both the parameters and the body must appear only once")
}

func TestSchemaToJSONSchemaIncludesRequiredForObjectProperties(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "CreateInventoryBatch",
			Method:      "POST",
			Path:        "/api/inventory/batch",
			Summary:     "在庫アイテムをまとめて作成する",
			RequestBody: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"items": {
						Type: domain.SchemaTypeArray,
						Items: &domain.Schema{
							Type:     domain.SchemaTypeObject,
							Required: []string{"name"},
							Properties: map[string]domain.Schema{
								"name": {Type: domain.SchemaTypeString},
							},
						},
					},
				},
			},
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 4)
	properties, ok := tools[0].InputSchema["properties"].(map[string]any)
	require.True(t, ok)

	items, ok := properties["items"].(map[string]any)
	require.True(t, ok)

	itemSchema, ok := items["items"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"name"}, itemSchema["required"],
		"a nested object schema's Required must reach the JSON Schema as required")
}

func TestToolsForNestedObjectAndArraySchemasRecurse(t *testing.T) {
	c := domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:     "inventory",
			OperationID: "CreateInventoryBatch",
			Method:      "POST",
			Path:        "/api/inventory/batch",
			Summary:     "在庫アイテムをまとめて作成する",
			RequestBody: &domain.Schema{
				Type: domain.SchemaTypeObject,
				Properties: map[string]domain.Schema{
					"items": {
						Type:        domain.SchemaTypeArray,
						Title:       "Items",
						Description: "the items to create",
						Items: &domain.Schema{
							Type: domain.SchemaTypeObject,
							Properties: map[string]domain.Schema{
								"name": {Type: domain.SchemaTypeString},
							},
						},
					},
				},
			},
		},
	}}

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 4)
	properties, ok := tools[0].InputSchema["properties"].(map[string]any)
	require.True(t, ok)

	items, ok := properties["items"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Items", items["title"])
	assert.Equal(t, "the items to create", items["description"])

	itemSchema, ok := items["items"].(map[string]any)
	require.True(t, ok, "an array's Items schema must be converted too")

	nestedProps, ok := itemSchema["properties"].(map[string]any)
	require.True(t, ok, "an object's Properties must be converted too")
	assert.Contains(t, nestedProps, "name")
}

func TestAskUserToolShape(t *testing.T) {
	tool := usecase.AskUserTool()

	assert.Equal(t, "ask_user", tool.Name)
	assert.True(t, tool.Strict)
	assert.NotEmpty(t, tool.Description)

	properties, ok := tool.InputSchema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, properties, "question")
	assert.Contains(t, properties, "operationId")
	assert.Contains(t, properties, "param")
	assert.Contains(t, properties, "options")

	// Defect 1 (docs/specs/shortlisting.md, measured 2026-09-15): with five
	// services offered, a free-text "service" argument tempted the model
	// into inventing one it never called ("approval", "salesBundle",
	// "summarize") - every one of those calls 500'd as
	// ErrEndpointNotFound. There is deliberately no "service" property at
	// all any more: the planner resolves it from operationId itself,
	// exactly as it already does for a real tool call (resolveService,
	// internal/adapter/planner/toolcall/planner.go).
	assert.NotContains(t, properties, "service")

	assert.ElementsMatch(t, []string{"question", "operationId", "param", "options"},
		tool.InputSchema["required"], "operationId must be required: a param name alone "+
			"is not unique across services")
}

// countByName returns how many of tools are named name - shared by every
// test below that only cares whether one tool appears, and how many times.
func countByName(tools []usecase.Tool, name string) int {
	count := 0

	for _, tool := range tools {
		if tool.Name == name {
			count++
		}
	}

	return count
}

// TestToolsForOffersNoProposePanelWithNoWorkspace is AC-O-101
// (docs/specs/offering.md): a question asked with no workspace id is
// offered no propose_panel at all - not present, not disabled, not
// described as unavailable (section 5's third exclusion) - and every other
// tool is unaffected.
func TestToolsForOffersNoProposePanelWithNoWorkspace(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{})

	// 3 catalogue endpoints + ask_user + list_capabilities, no propose_panel.
	require.Len(t, tools, 5)
	assert.Equal(t, 0, countByName(tools, "propose_panel"), "propose_panel must not be offered")
	assert.Equal(t, 1, countByName(tools, "ask_user"))
	assert.Equal(t, 1, countByName(tools, "list_capabilities"))
}

// TestToolsForOffersProposePanelWithAWorkspace is AC-O-102: a question
// asked from a workspace is offered propose_panel.
func TestToolsForOffersProposePanelWithAWorkspace(t *testing.T) {
	c := catalogWithEnumParameter()

	tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})

	require.Len(t, tools, 6)
	assert.Equal(t, 1, countByName(tools, "propose_panel"))
}

// TestToolsForAlwaysOffersAskUserAndListCapabilities is AC-O-103: a tool
// with no condition (ask_user, list_capabilities) is offered whether or
// not the request carries a workspace id.
func TestToolsForAlwaysOffersAskUserAndListCapabilities(t *testing.T) {
	c := catalogWithEnumParameter()

	for _, workspaceID := range []string{"", "ws-1"} {
		tools := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: workspaceID})

		assert.Equal(t, 1, countByName(tools, "ask_user"), "workspaceID=%q", workspaceID)
		assert.Equal(t, 1, countByName(tools, "list_capabilities"), "workspaceID=%q", workspaceID)
	}
}

// TestToolsForCatalogueToolsAreUnaffectedByPlanContext is AC-O-105: the
// catalogue's own operation tools - everything ToolsFor appends before its
// built-ins - are unchanged by PlanContext, the same tools in the same
// order, regardless of which built-in condition holds. assert.Equal on the
// slices themselves (not a wire re-encoding: internal/usecase may not
// import encoding/json at all, depguard) already proves both same content
// and same order - a reordering would fail the slice-index-by-index
// comparison testify's ObjectsAreEqual/reflect.DeepEqual performs here,
// exactly as internal/adapter/planner/toolcall's own byte-identical test
// proves it at the wire, where map key order can otherwise hide a shift.
func TestToolsForCatalogueToolsAreUnaffectedByPlanContext(t *testing.T) {
	c := catalogWithEnumParameter()
	catalogueCount := len(c.Endpoints)

	withWorkspace := usecase.ToolsFor(c, usecase.PlanContext{WorkspaceID: "ws-1"})
	withoutWorkspace := usecase.ToolsFor(c, usecase.PlanContext{})

	assert.Equal(t, withoutWorkspace[:catalogueCount], withWorkspace[:catalogueCount],
		"catalogue tools must be identical, in the same order, regardless of PlanContext")
}
