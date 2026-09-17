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

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// The jev picker's own app-wiring tests, split out from app_test.go
// (harness/quality/filelen.sh's 1000-line guard - app_test.go was already
// at 1007 lines with these here).

// fixtureJevServer answers POST /v1/systemone with a canned "none" choice,
// regardless of the request body - enough to prove the wiring holds end
// to end (docs/specs/staging.md, S1/S7), not what a real Jev call would
// answer.
func fixtureJevServer(t *testing.T) *httptest.Server {
	t.Helper()

	const response = `{
    "model": "jev-1.13.0",
    "answers": {"pick": {"type": "choice", "choice": "none", "confidence": 0.9}},
    "usage": {"input_tokens": 1, "output_tokens": 1}
  }`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// TestNewWithPickerJevBuildsAndServesAQuestion is the jev picker's own
// app-wiring test, mirroring TestNewWithLLMStagesTwoBuildsAndServesAQuestion
// (app_test.go): a Config with LLM.Stages: 2 and Picker.Name "jev" builds
// without error - jev.New is wired alongside the toolcall planner, called
// instead of pick.New - and answers a question over /api/plan.
func TestNewWithPickerJevBuildsAndServesAQuestion(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)
	jevServer := fixtureJevServer(t)

	handler, err := app.New(&app.Config{
		Services:      []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:           app.LLM{BaseURL: chatServer.URL, Model: "test-model", Stages: 2},
		Picker:        app.Picker{Name: app.PickerJev, JevAPIKey: "test-key", JevBaseURL: jevServer.URL},
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
}

// TestNewWithPickerJevCriteriaV2BuildsAndServesAQuestion mirrors
// TestNewWithPickerJevBuildsAndServesAQuestion with Picker.JevCriteria set
// to app.JevCriteriaV2, proving the wiring from Config down to
// jev.WithCriteria holds for the trial's second round too, not just the
// zero-value ("" -> CriteriaV1) path the test above already covers.
func TestNewWithPickerJevCriteriaV2BuildsAndServesAQuestion(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)
	jevServer := fixtureJevServer(t)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:      app.LLM{BaseURL: chatServer.URL, Model: "test-model", Stages: 2},
		Picker: app.Picker{
			Name: app.PickerJev, JevAPIKey: "test-key", JevBaseURL: jevServer.URL, JevCriteria: app.JevCriteriaV2,
		},
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
}

// TestNewRejectsAnUnknownPicker mirrors TestNewRejectsAnUnknownLLMMode
// (app_test.go): Config.Picker.Name is the same defence-in-depth as
// Config.LLM.Mode - cmd/api never reaches this path because
// internal/infra/config.Load already validates ORCHESTRA_PICKER, but a
// caller that builds a Config directly (as every test in this file does)
// gets the same refusal.
func TestNewRejectsAnUnknownPicker(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{BaseURL: "http://127.0.0.1:0", Stages: 2},
		Picker:        app.Picker{Name: "not-a-real-picker"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrInvalidPicker)
}

// TestNewRejectsPickerJevWithoutAnAPIKey mirrors TestNewRejectsAnUnknownPicker:
// a Config built directly with Picker.Name "jev" and no JevAPIKey gets the
// same refusal config.Load's own ErrMissingJevAPIKey already gives
// ORCHESTRA_PICKER=jev with no ORCHESTRA_JEV_API_KEY.
func TestNewRejectsPickerJevWithoutAnAPIKey(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{BaseURL: "http://127.0.0.1:0", Stages: 2},
		Picker:        app.Picker{Name: app.PickerJev},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrMissingJevAPIKey)
}
