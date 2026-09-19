package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// The pick stage's own named wording sets (Config.Picker.PickWording,
// ORCHESTRA_PICK_WORDING), split from app_picker_test.go the same way that
// file was split from app_test.go.

// TestNewRejectsAnUnknownPickWording mirrors
// TestNewRejectsAnUnknownPlannerWording: Config.Picker.PickWording is the
// same defence-in-depth as Config.LLM.Wording - cmd/api never reaches this
// path because internal/infra/config.Load already validates
// ORCHESTRA_PICK_WORDING, but a caller that builds a Config directly gets
// the same refusal. LLM.Stages must be 2 (the pick-then-fill path,
// docs/specs/staging.md S1) - PickWording is otherwise never resolved at
// all, the same reasoning Config.Picker.Name's own doc comment gives.
func TestNewRejectsAnUnknownPickWording(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{BaseURL: "http://127.0.0.1:0", Stages: 2},
		Picker:        app.Picker{PickWording: "not-a-real-wording"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrInvalidPickWording)
}

// TestNewWithPickWordingV2StrictCapabilitiesSendsItsOwnPhrasing is the
// wiring test: a Config naming "v2-strict-capabilities" builds without
// error and the pick request it sends carries that set's own
// ListCapabilities phrasing, not v1's.
func TestNewWithPickWordingV2StrictCapabilitiesSendsItsOwnPhrasing(t *testing.T) {
	fixture := fixtureService(t)
	capturing := fixtureChatServerCapturingRequests(t)

	handler, err := app.New(&app.Config{
		Services:      []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:           app.LLM{BaseURL: capturing.server.URL, Model: "test-model", Stages: 2},
		Picker:        app.Picker{PickWording: "v2-strict-capabilities"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	raw, err := json.Marshal(map[string]string{"query": "widgets please"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, *capturing.requests, "the pick must have sent at least one request")

	v2, ok := pick.WordingByName("v2-strict-capabilities")
	require.True(t, ok)

	pickRequest := (*capturing.requests)[0]
	messages, ok := pickRequest["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	user, ok := messages[1].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, user["content"], v2.ListCapabilities)
}
