package jev_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// statusEnumEndpoint is one safe operation with one enum-valued query
// parameter, "status" - the shape arm 2 (ORCHESTRA_FILL_ENUM=jev) is built
// for.
func statusEnumEndpoint() domain.Endpoint {
	return domain.Endpoint{
		Service:     "inventory",
		OperationID: "listInventoryItems",
		Method:      domain.MethodGet,
		Path:        "/items",
		Parameters: []domain.Parameter{
			{
				Name: "status", In: "query",
				Schema: domain.Schema{
					Type:       domain.SchemaTypeString,
					Enum:       []string{"in_stock", "quarantined"},
					EnumLabels: map[string]string{"in_stock": "入荷済み", "quarantined": "検品保留"},
					Title:      "ステータス",
				},
			},
		},
	}
}

// fillResponseBody builds a fixed wireResponse JSON body for "status"
// alone, the one question statusEnumEndpoint's single parameter produces -
// hand-built, the same reason gate_test.go's own noulResponseBody is,
// rather than json.Marshal over a handful of fixed literal values this
// file calls it with.
func fillResponseBody(choice string, confidence float64) string {
	c := strconv.FormatFloat(confidence, 'f', -1, 64)

	return `{"model":"jev-latest","answers":{"status":{"type":"choice","choice":"` + choice +
		`","confidence":` + c + `,"probabilities":{"` + choice + `":` + c +
		`}}},"usage":{"input_tokens":20,"output_tokens":3}}`
}

// fakeRequest is fakeRequestServer's own result: server is the running
// fixture; gotBody decodes the last request it received - a named result
// pair, not two bare returns, so gocritic's unnamedResult
// (harness/quality/go/golangci.yml) has nothing to flag and a caller reads
// .server/.gotBody by name rather than by position.
type fakeRequest struct {
	server  *httptest.Server
	gotBody func() map[string]any
}

// fakeRequestServer starts an httptest.Server answering every request with
// body, and returns a getter for the last request's own decoded JSON -
// captured through a closure, not a pointer-to-map parameter, so a caller
// never has to hand this function a pointer to a reference type.
func fakeRequestServer(t *testing.T, body string) fakeRequest {
	t.Helper()

	var lastBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		lastBody = raw

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return fakeRequest{server: server, gotBody: func() map[string]any {
		var decoded map[string]any
		if err := json.Unmarshal(lastBody, &decoded); err != nil {
			t.Errorf("decoding captured request body: %v", err)
		}

		return decoded
	}}
}

// fakeServer starts an httptest.Server answering every request with body,
// for a test that never inspects the request itself.
func fakeServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	return fakeRequestServer(t, body).server
}

// TestFillerRequestShapeOneQuestionPerParameterWithBothSentinels is the
// request shape golden test: one Choice question keyed by the parameter's
// own name, its criteria holding every enum value (described by its own
// x-enum-labels label) plus both sentinels.
func TestFillerRequestShapeOneQuestionPerParameterWithBothSentinels(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	gotBody := fake.gotBody()

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok, "questions must be an object")
	require.Len(t, questions, 1, "one question per parameter")

	status, ok := questions["status"].(map[string]any)
	require.True(t, ok, "the one question must be keyed by the parameter's own name")
	assert.Equal(t, "choice", status["type"])

	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "入荷済み", criteria["in_stock"], "enum value described by its own x-enum-labels label")
	assert.Equal(t, "検品保留", criteria["quarantined"])
	assert.Equal(t, "この質問はこの項目を絞り込んでいない", criteria["__unset__"])
	assert.Equal(t, "質問はこの項目を絞り込んでいるが、候補の値のどれとも一致しない", criteria["__mismatch__"])
	assert.Len(t, criteria, 4, "two enum values plus both sentinels, nothing else")

	assert.Equal(t, "jev-latest", gotBody["model"])
	assert.Contains(t, gotBody["state"], "在庫を見せて", "state carries the question text")
}

// TestFillerRequestOmitsRefusalByDefault proves ORCHESTRA_FILL_ENUM_REFUSAL's
// own sentinel is absent from a plain Filler's criteria - the request stays
// byte-identical to before this option existed.
func TestFillerRequestOmitsRefusalByDefault(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	status, ok := fake.gotBody()["questions"].(map[string]any)["status"].(map[string]any)
	require.True(t, ok)
	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)

	assert.NotContains(t, criteria, "__refusal__", "WithFillRefusal was never given")
	assert.NotContains(t, criteria, "__capabilities__", "WithFillRefusal was never given")
	assert.Len(t, criteria, 4, "two enum values plus the two default sentinels, nothing else")
}

// TestFillerRequestIncludesRefusalWhenEnabled proves WithFillRefusal adds
// the third sentinel, described in Japanese, without naming any eval
// question.
func TestFillerRequestIncludesRefusalWhenEnabled(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	status, ok := fake.gotBody()["questions"].(map[string]any)["status"].(map[string]any)
	require.True(t, ok)
	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "選ばれた操作は、そもそもこの質問には答えられない（操作の対象や種類が質問と合っていない）", criteria["__refusal__"])
	assert.Len(t, criteria, 6, "two enum values plus all four sentinels")
}

// TestFillerRequestIncludesCapabilitiesWhenEnabled proves WithFillRefusal
// also adds capabilitiesChoice, worded distinctly from refusalChoice's own
// text and naming no eval question or service.
func TestFillerRequestIncludesCapabilitiesWhenEnabled(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	status, ok := fake.gotBody()["questions"].(map[string]any)["status"].(map[string]any)
	require.True(t, ok)
	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)

	capabilities, ok := criteria["__capabilities__"].(string)
	require.True(t, ok)
	assert.NotEqual(t, criteria["__refusal__"], capabilities, "distinct wording from refusalChoice")
	assert.Len(t, criteria, 6, "two enum values plus all four sentinels")
}

// TestFillerARefusalBecomesDecisionNone proves a __refusal__ answer on any
// parameter maps to the same bare usecase.Decision{Kind: DecisionNone} the
// local fill's own decline produces - resolvePickedFill turns either into
// the identical ResultKindNone shape.
func TestFillerARefusalBecomesDecisionNone(t *testing.T) {
	server := fakeServer(t, fillResponseBody("__refusal__", 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を全部消して", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionNone}, decision)
}

// TestFillerACapabilitiesAnswerBecomesDecisionListCapabilities proves a
// __capabilities__ answer on any parameter maps to the bare
// usecase.Decision{Kind: DecisionListCapabilities} - the identical shape
// resolvePickedFill's own DecisionListCapabilities case already expects
// from the local fill's own decline.
func TestFillerACapabilitiesAnswerBecomesDecisionListCapabilities(t *testing.T) {
	server := fakeServer(t, fillResponseBody("__capabilities__", 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫で何ができる？", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionListCapabilities}, decision)
}

// twoEnumParamEndpoint is one safe operation with two enum-valued
// parameters, "status" and "kind" - used only by the precedence test
// below, which needs two parameters to answer differently.
func twoEnumParamEndpoint() domain.Endpoint {
	endpoint := statusEnumEndpoint()
	endpoint.Parameters = append(endpoint.Parameters, domain.Parameter{
		Name: "kind", In: "query",
		Schema: domain.Schema{
			Type:       domain.SchemaTypeString,
			Enum:       []string{"raw", "finished"},
			EnumLabels: map[string]string{"raw": "原材料", "finished": "完成品"},
			Title:      "種別",
		},
	})

	return endpoint
}

// TestFillerRefusalOutranksCapabilitiesWhenBothWin proves the precedence
// order: when one parameter answers __refusal__ and another answers
// __capabilities__, the outcome is still DecisionNone - a claim about the
// picked operation being wrong for the question outranks a claim about the
// question being a capabilities question, the same ordering
// fillOutcome.decision's own doc comment explains.
func TestFillerRefusalOutranksCapabilitiesWhenBothWin(t *testing.T) {
	body := `{"model":"jev-latest","answers":{` +
		`"status":{"type":"choice","choice":"__refusal__","confidence":0.9,"probabilities":{"__refusal__":0.9}},` +
		`"kind":{"type":"choice","choice":"__capabilities__","confidence":0.9,"probabilities":{"__capabilities__":0.9}}` +
		`},"usage":{"input_tokens":20,"output_tokens":3}}`
	server := fakeServer(t, body)

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := twoEnumParamEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫で何ができる？", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionNone}, decision)
}

// TestFillerCapabilitiesOutranksMismatchWhenBothWin is the precedence
// order's other pair: when one parameter answers __capabilities__ and
// another answers __mismatch__, the outcome is DecisionListCapabilities,
// not DecisionAsk - a claim about the whole request outranks a claim about
// one field's value.
func TestFillerCapabilitiesOutranksMismatchWhenBothWin(t *testing.T) {
	body := `{"model":"jev-latest","answers":{` +
		`"status":{"type":"choice","choice":"__mismatch__","confidence":0.9,"probabilities":{"__mismatch__":0.9}},` +
		`"kind":{"type":"choice","choice":"__capabilities__","confidence":0.9,"probabilities":{"__capabilities__":0.9}}` +
		`},"usage":{"input_tokens":20,"output_tokens":3}}`
	server := fakeServer(t, body)

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := twoEnumParamEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫で何ができる？", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionListCapabilities}, decision)
}

// TestFillerRequestUnsetWordingDefaultsToNarrow proves the __unset__
// criterion stays byte-identical to before ORCHESTRA_FILL_ENUM_UNSET_WORDING
// existed when the Filler is built without WithFillUnsetWordingWide.
func TestFillerRequestUnsetWordingDefaultsToNarrow(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	status, ok := fake.gotBody()["questions"].(map[string]any)["status"].(map[string]any)
	require.True(t, ok)
	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "この質問はこの項目を絞り込んでいない", criteria["__unset__"])
}

// TestFillerRequestUnsetWordingWideCoversDroppingAFilter proves
// WithFillUnsetWordingWide rewords __unset__ to additionally cover a
// question that explicitly asks for everything or removes an earlier
// turn's restriction (the d06 turn 2 regression) - without naming that
// eval question's own words.
func TestFillerRequestUnsetWordingWideCoversDroppingAFilter(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil, jev.WithFillUnsetWordingWide())
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	status, ok := fake.gotBody()["questions"].(map[string]any)["status"].(map[string]any)
	require.True(t, ok)
	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)

	wide, ok := criteria["__unset__"].(string)
	require.True(t, ok)
	assert.Contains(t, wide, "絞り込んでいない", "still covers the plain no-restriction case")
	assert.Contains(t, wide, "絞り込みを解除した場合を含む", "additionally covers dropping an earlier restriction")
}

// separateStatusConfidence is the fixed confidence separateResponseBody
// gives "status"'s own choice answer - every call site here varies only the
// choice and the two noul probabilities, never this, so it is not its own
// parameter (golangci's unparam).
const separateStatusConfidence = 0.9

// separateResponseBody builds a fixed wireResponse JSON body carrying
// "status"'s own choice answer plus the two whole-request "noul" answers
// refusalModeSeparate asks for (fillRefusalQuestionName,
// fillCapabilitiesQuestionName) - the response shape a real Jev deployment
// would send back for addSeparateRefusalQuestions' own request.
func separateResponseBody(statusChoice string, refusalNoul, capabilitiesNoul float64) string {
	sc := strconv.FormatFloat(separateStatusConfidence, 'f', -1, 64)
	rn := strconv.FormatFloat(refusalNoul, 'f', -1, 64)
	cn := strconv.FormatFloat(capabilitiesNoul, 'f', -1, 64)

	return `{"model":"jev-latest","answers":{` +
		`"status":{"type":"choice","choice":"` + statusChoice + `","confidence":` + sc +
		`,"probabilities":{"` + statusChoice + `":` + sc + `}},` +
		`"refusal":{"type":"noul","noul":` + rn + `},` +
		`"capabilities":{"type":"noul","noul":` + cn + `}` +
		`},"usage":{"input_tokens":25,"output_tokens":4}}`
}

// TestFillerSeparateRequestShapeAddsTwoNoulQuestionsNoSentinelsInOptions
// proves refusalModeSeparate's own request shape: one Choice question per
// parameter, carrying only its own field-level options (enum values,
// __unset__, __mismatch__ - never __refusal__/__capabilities__), plus two
// extra "noul" questions in the same request, one per whole-request
// judgement.
func TestFillerSeparateRequestShapeAddsTwoNoulQuestionsNoSentinelsInOptions(t *testing.T) {
	fake := fakeRequestServer(t, separateResponseBody("in_stock", 0.1, 0.1))

	f := jev.NewFiller(fake.server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	questions, ok := fake.gotBody()["questions"].(map[string]any)
	require.True(t, ok)
	require.Len(t, questions, 3, "one Choice question per parameter plus the two noul questions")

	status, ok := questions["status"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "choice", status["type"])

	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, criteria, "__refusal__", "a whole-request claim never competes inside a field's own options")
	assert.NotContains(t, criteria, "__capabilities__")
	assert.Len(t, criteria, 4, "two enum values plus __unset__/__mismatch__ only")

	refusal, ok := questions["refusal"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "noul", refusal["type"])
	assert.Equal(t, refusalLabelForTest, refusal["instructions"])
	assertCriteriaNamePickedOperation(t, refusal["criteria"])

	capabilities, ok := questions["capabilities"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "noul", capabilities["type"])
	assert.Equal(t, capabilitiesLabelForTest, capabilities["instructions"])
	assertCriteriaNamePickedOperation(t, capabilities["criteria"])
}

// assertCriteriaNamePickedOperation asserts raw decodes as a true/false
// criteria object (docs.typesafe.ai/primitives/noul) whose own texts both
// name statusEnumEndpoint's own operation id - the evidence
// operationEvidenceFor (filler_mapping.go) embeds so the judgement has
// something to weigh beyond its own instruction sentence
// (docs/measurements: the first cut of refusalModeSeparate's own noul
// questions sat in a narrow probability band regardless of the picked
// operation, because they carried no evidence about it at all).
// statusEnumEndpoint declares no DisplayName, so DisplayNameOr falls back to
// its own OperationID ("listInventoryItems") - TestFillerSeparateCriteriaNameOperationDisplayName
// below checks the DisplayName branch with a fuller endpoint.
func assertCriteriaNamePickedOperation(t *testing.T, raw any) {
	t.Helper()

	criteria, ok := raw.(map[string]any)
	require.True(t, ok, "criteria must be a true/false object, not left unset")
	require.Len(t, criteria, 2)

	for _, key := range []string{"true", "false"} {
		text, ok := criteria[key].(string)
		require.True(t, ok, "criteria[%q] must be a string", key)
		assert.Contains(t, text, "listInventoryItems", "criteria[%q] must name the operation id", key)
	}
}

// TestFillerSeparateCriteriaNameOperationDisplayName proves
// operationEvidenceFor's own DisplayName/ServiceDisplayName/Summary branches:
// a fuller endpoint than statusEnumEndpoint (a real ListInventoryItems-shaped
// one, docs/measurements) makes both noul questions' own criteria name the
// operation's display name and its service's display name, not only its bare
// operation id.
func TestFillerSeparateCriteriaNameOperationDisplayName(t *testing.T) {
	fake := fakeRequestServer(t, separateResponseBody("in_stock", 0.1, 0.1))

	f := jev.NewFiller(fake.server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := domain.Endpoint{
		Service:            "inventory",
		OperationID:        "ListInventoryItems",
		Method:             domain.MethodGet,
		Path:               "/items",
		Summary:            "List stock items, optionally filtered by status.",
		DisplayName:        "在庫一覧",
		ServiceDisplayName: "在庫管理",
		Parameters: []domain.Parameter{
			{
				Name: "status", In: "query",
				Schema: domain.Schema{
					Type:       domain.SchemaTypeString,
					Enum:       []string{"in_stock", "quarantined"},
					EnumLabels: map[string]string{"in_stock": "入荷済み", "quarantined": "検品保留"},
					Title:      "ステータス",
				},
			},
		},
	}

	_, _, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)

	questions, ok := fake.gotBody()["questions"].(map[string]any)
	require.True(t, ok)

	for _, name := range []string{"refusal", "capabilities"} {
		criteria, ok := questions[name].(map[string]any)["criteria"].(map[string]any)
		require.True(t, ok)

		for _, key := range []string{"true", "false"} {
			text, ok := criteria[key].(string)
			require.True(t, ok)
			assert.Contains(t, text, "ListInventoryItems", "%s[%q] names the operation id", name, key)
			assert.Contains(t, text, "在庫一覧", "%s[%q] names the operation's display name", name, key)
			assert.Contains(t, text, "在庫管理", "%s[%q] names the service display name", name, key)
		}
	}
}

// refusalLabelForTest and capabilitiesLabelForTest mirror filler.go's own
// unexported refusalLabel/capabilitiesLabel - this test package (jev_test)
// cannot reach those directly, and the golden request-shape assertion above
// wants the literal text, not merely "is non-empty", so the report can quote
// it verbatim.
const (
	refusalLabelForTest      = "選ばれた操作は、そもそもこの質問には答えられない（操作の対象や種類が質問と合っていない）"
	capabilitiesLabelForTest = "質問は特定の操作の実行を求めているのではなく、そもそもこの仕組み全体で何ができるか、どんな操作があるかを尋ねている"
)

// TestFillerSeparateRefusalAboveThresholdBecomesDecisionNone proves the
// refusal noul question's own answer, at or above threshold, maps to
// DecisionNone - the same outcome refusalModeInOptions' own __refusal__
// answer produces, reached a different way.
func TestFillerSeparateRefusalAboveThresholdBecomesDecisionNone(t *testing.T) {
	server := fakeServer(t, separateResponseBody("__unset__", 0.7, 0.1))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を削除したい", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionNone}, decision)
}

// TestFillerSeparateRefusalBelowThresholdDoesNotFire proves the refusal
// noul question's own answer, below threshold, never fires - the plain call
// goes through when every parameter also answered __unset__.
func TestFillerSeparateRefusalBelowThresholdDoesNotFire(t *testing.T) {
	server := fakeServer(t, separateResponseBody("__unset__", 0.49, 0.1))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を減らしたい", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
}

// TestFillerSeparateCapabilitiesAboveThresholdBecomesDecisionListCapabilities
// proves the capabilities noul question's own answer, at or above threshold,
// maps to DecisionListCapabilities when refusal did not also fire.
func TestFillerSeparateCapabilitiesAboveThresholdBecomesDecisionListCapabilities(t *testing.T) {
	server := fakeServer(t, separateResponseBody("__unset__", 0.1, 0.96))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫で何ができる？", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionListCapabilities}, decision)
}

// TestFillerSeparateRefusalOutranksCapabilitiesWhenBothFire proves the same
// precedence order refusalModeInOptions already has, reached through the two
// separate noul answers instead of two parameters' own sentinel answers.
func TestFillerSeparateRefusalOutranksCapabilitiesWhenBothFire(t *testing.T) {
	server := fakeServer(t, separateResponseBody("__unset__", 0.8, 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫で何ができる？", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionNone}, decision)
}

// TestFillerSeparateCapabilitiesOutranksMismatchWhenBothFire proves a
// whole-request separate judgement still outranks a single parameter's own
// __mismatch__ answer.
func TestFillerSeparateCapabilitiesOutranksMismatchWhenBothFire(t *testing.T) {
	server := fakeServer(t, separateResponseBody("__mismatch__", 0.1, 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫で何ができる？", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionListCapabilities}, decision)
}

// TestFillerSeparateMismatchStillAsksWhenNeitherJudgementFires proves the
// mismatch path is unaffected by refusalModeSeparate when neither
// whole-request judgement fires - today's ask still happens.
func TestFillerSeparateMismatchStillAsksWhenNeitherJudgementFires(t *testing.T) {
	server := fakeServer(t, separateResponseBody("__mismatch__", 0.1, 0.1))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "だめなやつを見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionAsk, decision.Kind)
	assert.Equal(t, "status", decision.Param)
}

// TestFillerSeparateFailsOpenWhenRefusalAnswerMissing proves a response
// missing the "refusal" noul answer entirely fails open, the same way a
// missing per-parameter answer already does - a response that does not
// answer what refusalModeSeparate asked for is not trustworthy enough to
// build a decision from.
func TestFillerSeparateFailsOpenWhenRefusalAnswerMissing(t *testing.T) {
	body := `{"model":"jev-latest","answers":{` +
		`"status":{"type":"choice","choice":"in_stock","confidence":0.9,"probabilities":{"in_stock":0.9}},` +
		`"capabilities":{"type":"noul","noul":0.1}` +
		`},"usage":{"input_tokens":20,"output_tokens":3}}`
	server := fakeServer(t, body)

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillRefusalSeparate())
	endpoint := statusEnumEndpoint()

	_, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestFillerModeOneStillBehavesExactlyAsBeforeSeparateExisted proves
// WithFillRefusal (mode "1") is unaffected by refusalModeSeparate's own
// existence: same in-options criteria, same __refusal__/__capabilities__
// mapping, no noul questions added.
func TestFillerModeOneStillBehavesExactlyAsBeforeSeparateExisted(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("__refusal__", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil, jev.WithFillRefusal())
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を全部消して", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, usecase.Decision{Kind: usecase.DecisionNone}, decision)

	questions, ok := fake.gotBody()["questions"].(map[string]any)
	require.True(t, ok)
	require.Len(t, questions, 1, "no extra noul questions in mode 1")

	status, ok := questions["status"].(map[string]any)
	require.True(t, ok)
	criteria, ok := status["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, criteria, 6, "two enum values plus all four sentinels, unchanged")
}

// TestFillerRequestStateCarriesTurns proves state is built from the same
// helper the local fill's own toolcall.Planner renders turns with -
// TestFillerRequestShapeOneQuestionPerParameterWithBothSentinels already
// covers the query alone; this covers a prior turn appearing in state too.
func TestFillerRequestStateCarriesTurns(t *testing.T) {
	fake := fakeRequestServer(t, fillResponseBody("in_stock", 0.9))

	f := jev.NewFiller(fake.server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	turns := []usecase.Turn{{Question: "在庫は？", Kind: usecase.ResultKindResult, Service: "inventory", OperationID: "listInventoryItems"}}

	_, _, err := f.Fill(context.Background(), &endpoint, "検品中のは？", nil, turns, usecase.PlanContext{})
	require.NoError(t, err)

	state, ok := fake.gotBody()["state"].(string)
	require.True(t, ok)
	assert.Contains(t, state, "在庫は？", "the prior turn's own question text")
	assert.Contains(t, state, "検品中のは？")
}

// TestFillerEveryParameterUnsetIsACallWithNoArguments is the "answers
// map" test: every parameter answering __unset__ becomes a DecisionCall
// with no arguments, the same outcome arm 1 reaches directly.
func TestFillerEveryParameterUnsetIsACallWithNoArguments(t *testing.T) {
	server := fakeServer(t, fillResponseBody("__unset__", 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "listInventoryItems", decision.OperationID)
	assert.Nil(t, decision.Args, "no answered parameter means no arguments at all")
}

// TestFillerAMatchedValueBecomesThatArgument is the "answers map" test's
// other half: a parameter answering one of its own enum values becomes
// that argument on the DecisionCall.
func TestFillerAMatchedValueBecomesThatArgument(t *testing.T) {
	server := fakeServer(t, fillResponseBody("quarantined", 0.95))

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "検品保留のを見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionCall, decision.Kind)
	assert.Equal(t, map[string]any{"status": "quarantined"}, decision.Args)
}

// TestFillerAMismatchBecomesAskOverThatParameter is the mismatch mapping
// test: a __mismatch__ answer becomes a DecisionAsk over that same
// parameter, worded the same way usecase.askForEnumGuess's own question is
// ("<title>はどれですか？") so Orchestrator.ask produces the identical
// ResultKindAsk shape either way.
func TestFillerAMismatchBecomesAskOverThatParameter(t *testing.T) {
	server := fakeServer(t, fillResponseBody("__mismatch__", 0.9))

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "だめなやつを見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, usecase.DecisionAsk, decision.Kind)
	assert.Equal(t, "inventory", decision.Service)
	assert.Equal(t, "listInventoryItems", decision.OperationID)
	assert.Equal(t, "status", decision.Param)
	assert.Equal(t, "ステータスはどれですか？", decision.Question)
}

// TestFillerFailsOpenOnLowConfidence proves an answer's confidence below
// the configured threshold fails open: ok is false, err is nil - never an
// error the caller must act on.
func TestFillerFailsOpenOnLowConfidence(t *testing.T) {
	server := fakeServer(t, fillResponseBody("in_stock", 0.4))

	f := jev.NewFiller(server.URL, "test-key", nil, jev.WithFillThreshold(0.5))
	endpoint := statusEnumEndpoint()

	decision, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, usecase.Decision{}, decision)
}

// TestFillerFailsOpenOnMissingAnswer proves a response naming no answer at
// all for a parameter's own question fails open the same way a
// low-confidence one does.
func TestFillerFailsOpenOnMissingAnswer(t *testing.T) {
	body := `{"model":"jev-latest","answers":{},"usage":{"input_tokens":5,"output_tokens":1}}`
	server := fakeServer(t, body)

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestFillerFailsOpenOnTransportError proves a non-2xx response fails
// open too, this time with a non-nil error the caller logs but never
// surfaces to the person asking.
func TestFillerFailsOpenOnTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	f := jev.NewFiller(server.URL, "test-key", nil)
	endpoint := statusEnumEndpoint()

	_, ok, err := f.Fill(context.Background(), &endpoint, "在庫を見せて", nil, nil, usecase.PlanContext{})
	require.Error(t, err)
	assert.False(t, ok)
}
