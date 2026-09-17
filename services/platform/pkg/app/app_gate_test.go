package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// The v3 Jev trial's own app-wiring tests (docs/measurements/jev-picker-v3.md),
// split out the same way app_picker_test.go was split from app_test.go
// (harness/quality/filelen.sh's 1000-line guard).

// fixtureJevGateServer answers POST /v1/systemone with a canned "noul"
// answer (fixed at noul, regardless of the request body) - enough to
// prove the wiring holds end to end, not what a real Jev call would
// answer. Unlike fixtureJevServer (the picker's own fixture), this is not
// shared with it: a staged plan under Gate wiring calls this endpoint
// twice per request (gate, then pick) against two different servers in
// the tests below, so each needs its own canned shape.
func fixtureJevGateServer(t *testing.T, noul float64) *httptest.Server {
	t.Helper()

	response := `{"model":"jev-1.13.0","answers":{"gate":{"type":"noul","noul":` +
		strconv.FormatFloat(noul, 'f', -1, 64) + `}},"usage":{"input_tokens":1,"output_tokens":1}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// TestNewWithGateJevBuildsAndServesAQuestion mirrors
// TestNewWithPickerJevBuildsAndServesAQuestion: a Config with LLM.Stages:
// 2 and Gate.Name "jev" (Possible, noul below the default threshold)
// builds without error and still answers a question over /api/plan - the
// gate runs ahead of the local picker, refuses nothing, and the request
// resolves exactly as it would with no gate configured.
func TestNewWithGateJevBuildsAndServesAQuestion(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)
	gateServer := fixtureJevGateServer(t, 0.1)

	handler, err := app.New(&app.Config{
		Services:      []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:           app.LLM{BaseURL: chatServer.URL, Model: "test-model", Stages: 2},
		Gate:          app.Gate{Name: app.GateJev, JevAPIKey: "test-key", JevBaseURL: gateServer.URL},
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

// TestNewWithGateJevImpossibleAnswersNoneWithoutThePlanner proves the
// gate's own reason to exist reaches all the way through New's wiring: a
// noul at or above the default threshold answers none over /api/plan
// without the stub planner (fixtureChatServer) ever being called - if it
// were, that server would answer whatever fixtureChatServer's own fixed
// tool-call response says, not none.
func TestNewWithGateJevImpossibleAnswersNoneWithoutThePlanner(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)
	gateServer := fixtureJevGateServer(t, 0.95)

	handler, err := app.New(&app.Config{
		Services:      []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:           app.LLM{BaseURL: chatServer.URL, Model: "test-model", Stages: 2},
		Gate:          app.Gate{Name: app.GateJev, JevAPIKey: "test-key", JevBaseURL: gateServer.URL},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	raw, err := json.Marshal(map[string]string{"query": "在庫を集計したい"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "none", body["kind"])
}

// fixtureJevFanOutServer answers POST /v1/systemone with both a "pick"
// choice answer and an "impossible" noul answer, regardless of the
// request body - the v5 trial's own fan-out wiring
// (docs/measurements/jev-picker-v5.md): Picker.Name and Gate.Name both
// "jev" fold the gate question into the same request the picker sends,
// so a single server (not fixtureJevServer plus fixtureJevGateServer
// against two different URLs) is enough to prove the wiring holds.
func fixtureJevFanOutServer(t *testing.T, noul float64) *httptest.Server {
	t.Helper()

	response := `{"model":"jev-1.13.0","answers":{` +
		`"pick":{"type":"choice","choice":"none","confidence":0.9},` +
		`"impossible":{"type":"noul","noul":` + strconv.FormatFloat(noul, 'f', -1, 64) + `}` +
		`},"usage":{"input_tokens":1,"output_tokens":1}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// TestNewWithPickerJevAndGateJevFoldsTheGateIntoTheOnePickRequest is the
// v5 trial's own app-wiring test (docs/measurements/jev-picker-v5.md,
// "fan-out, not extra calls"): a Config with both Picker.Name and
// Gate.Name "jev" answers a question over /api/plan through one Jev
// server alone - there is no second server for a standalone gate to call,
// so a passing request proves stagingOptions skipped newGate/
// usecase.WithGate entirely for this combination and folded the gate
// question into jev.Picker's own request instead (pkg/app/app_staging.go).
func TestNewWithPickerJevAndGateJevFoldsTheGateIntoTheOnePickRequest(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)
	jevServer := fixtureJevFanOutServer(t, 0.1)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:      app.LLM{BaseURL: chatServer.URL, Model: "test-model", Stages: 2},
		Picker:   app.Picker{Name: app.PickerJev, JevAPIKey: "test-key", JevBaseURL: jevServer.URL},
		Gate:     app.Gate{Name: app.GateJev, JevAPIKey: "test-key", JevBaseURL: jevServer.URL},
		DBPath:   filepath.Join(t.TempDir(), "app.db"), AdminPassword: appTestAdminPassword,
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

// TestNewWithPickerJevAndGateJevImpossibleAnswersNoneWithoutThePlanner
// mirrors TestNewWithGateJevImpossibleAnswersNoneWithoutThePlanner for the
// fan-out combination: a noul at or above the default threshold, answered
// alongside the "pick" question in the one fan-out request, still
// answers none over /api/plan without the stub planner ever being called.
func TestNewWithPickerJevAndGateJevImpossibleAnswersNoneWithoutThePlanner(t *testing.T) {
	fixture := fixtureService(t)
	chatServer := fixtureChatServer(t)
	jevServer := fixtureJevFanOutServer(t, 0.95)

	handler, err := app.New(&app.Config{
		Services: []app.Service{{Name: "fixture", URL: fixture.URL}},
		LLM:      app.LLM{BaseURL: chatServer.URL, Model: "test-model", Stages: 2},
		Picker:   app.Picker{Name: app.PickerJev, JevAPIKey: "test-key", JevBaseURL: jevServer.URL},
		Gate:     app.Gate{Name: app.GateJev, JevAPIKey: "test-key", JevBaseURL: jevServer.URL},
		DBPath:   filepath.Join(t.TempDir(), "app.db"), AdminPassword: appTestAdminPassword,
	})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	signInTestAdmin(t, server)

	raw, err := json.Marshal(map[string]string{"query": "在庫を集計したい"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/plan", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "none", body["kind"])
}

// TestNewRejectsAnUnknownGate mirrors TestNewRejectsAnUnknownPicker.
func TestNewRejectsAnUnknownGate(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{BaseURL: "http://127.0.0.1:0", Stages: 2},
		Gate:          app.Gate{Name: "not-a-real-gate"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrInvalidGate)
}

// TestNewRejectsGateJevWithoutAnAPIKey mirrors
// TestNewRejectsPickerJevWithoutAnAPIKey.
func TestNewRejectsGateJevWithoutAnAPIKey(t *testing.T) {
	_, err := app.New(&app.Config{
		LLM:           app.LLM{BaseURL: "http://127.0.0.1:0", Stages: 2},
		Gate:          app.Gate{Name: app.GateJev},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrMissingJevAPIKey)
}
