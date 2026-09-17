package jev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// collisionShortlist is two endpoints on two services sharing the same
// DisplayName ("承認") - the mechanical collision notFor detects - plus
// one x-orchestra-examples entry on the first, so a single shortlist
// exercises both CriteriaV2 fields the plain v1 line never carried.
func collisionShortlist() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:            "purchasing",
			OperationID:        "createPurchasingApproval",
			ServiceDisplayName: "購買管理",
			Summary:            "発注の承認を登録する",
			DisplayName:        "承認",
			Examples:           []string{"発注を承認したい", "発注の承認状況を見たい"},
		},
		{
			Service:            "attendance",
			OperationID:        "createAttendanceApproval",
			ServiceDisplayName: "勤怠管理",
			Summary:            "休暇申請の承認を登録する",
			DisplayName:        "承認",
		},
	}}
}

// TestPickWithCriteriaV2SendsWhatExamplesAndNotFor is this trial's own
// "the v2 request body exact for a two-endpoint shortlist with a
// collision and examples": both shortlist entries carry the shared
// DisplayName "承認" across two different services, so each must name
// the other in its own "not_for", and the entry with x-orchestra-examples
// must carry them as its own "examples".
func TestPickWithCriteriaV2SendsWhatExamplesAndNotFor(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(
			`{"model":"jev-1.13.0","answers":{"pick":{"type":"choice","choice":"createPurchasingApproval",` +
				`"confidence":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil, jev.WithCriteria(jev.CriteriaV2))

	_, err := picker.Pick(context.Background(), "発注を承認したい", nil, collisionShortlist())
	require.NoError(t, err)

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, pickQuestion["instructions"], "examples")
	assert.Contains(t, pickQuestion["instructions"], "not_for")

	criteria, ok := pickQuestion["criteria"].(map[string]any)
	require.True(t, ok)

	purchasing, ok := criteria["createPurchasingApproval"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "購買管理 / 発注の承認を登録する", purchasing["what"])
	assert.Equal(t, []any{"発注を承認したい", "発注の承認状況を見たい"}, purchasing["examples"])
	assert.Equal(t, "勤怠管理の承認ではない", purchasing["not_for"])

	attendance, ok := criteria["createAttendanceApproval"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "勤怠管理 / 休暇申請の承認を登録する", attendance["what"])
	assert.NotContains(t, attendance, "examples")
	assert.Equal(t, "購買管理の承認ではない", attendance["not_for"])

	listCapabilities, ok := criteria[pick.IDListCapabilities].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pick.PhraseListCapabilities, listCapabilities["what"])
	assert.Equal(t, []any{"何ができるの？"}, listCapabilities["examples"])
	assert.NotContains(t, listCapabilities, "not_for")

	proposePanel, ok := criteria[pick.IDProposePanel].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pick.PhraseProposePanel, proposePanel["what"])
	assert.NotContains(t, proposePanel, "examples")

	none, ok := criteria[pick.IDNone].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pick.PhraseNone, none["what"])
	assert.Equal(t, []any{"今日の天気は？", "好きな食べ物は何？", "システムを再起動して"}, none["examples"])
}

// TestPickWithNoCriteriaOptionSendsCriteriaV1ByteForByte guards the
// default: a Picker built with no WithCriteria option at all (New's
// zero-value "criteria", the only path every caller before this trial's
// second round ever took) sends exactly the v1 line, never the v2
// object - so this trial's addition changes nothing for a deployment
// that never sets ORCHESTRA_JEV_CRITERIA.
func TestPickWithNoCriteriaOptionSendsCriteriaV1ByteForByte(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(
			`{"model":"jev-1.13.0","answers":{"pick":{"type":"choice","choice":"createPurchasingApproval",` +
				`"confidence":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	picker := jev.New(server.URL, "test-key", nil)

	_, err := picker.Pick(context.Background(), "発注を承認したい", nil, collisionShortlist())
	require.NoError(t, err)

	questions, ok := gotBody["questions"].(map[string]any)
	require.True(t, ok)

	pickQuestion, ok := questions["pick"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, pickQuestion["instructions"], "not_for")

	criteria, ok := pickQuestion["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "購買管理 / 発注の承認を登録する", criteria["createPurchasingApproval"])
	assert.Equal(t, "勤怠管理 / 休暇申請の承認を登録する", criteria["createAttendanceApproval"])
}
