package jsonmode_test

// jsonmode.WithClock's own tests (TODO.md's "invented form values" item,
// DECISIONS.md 2026-09-16 "Thirty questions against the real dev
// services") - this planner's own equivalent of
// toolcall/planner_clock_test.go. fixtureCatalog, chatContent and
// plannerFixture are defined in planner_test.go, same package.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jsonmode"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// newPlannerWithClock is newPlanner (planner_test.go) with a pinned clock:
// a separate helper, not an added parameter to newPlanner itself, since
// every other test in this package leaves the clock at New's own default
// and golangci-lint's unparam check (part of the fixed harness policy)
// rejects a parameter no caller varies.
func newPlannerWithClock(t *testing.T, catalog domain.Catalog, clock func() time.Time, body string) *plannerFixture {
	t.Helper()

	fixture := &plannerFixture{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var decoded map[string]any
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Errorf("decoding request body: %v", err)
		}

		fixture.requests = append(fixture.requests, decoded)

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fixture response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	fixture.planner = jsonmode.New(client, catalog, jsonmode.WithClock(clock))

	return fixture
}

// userContent reads request's user message (messages[len-1]) content -
// this package's own equivalent of toolcall_test.userContentOf.
func userContent(t *testing.T, request map[string]any) string {
	t.Helper()

	messages, ok := request["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)

	last, ok := messages[len(messages)-1].(map[string]any)
	require.True(t, ok)

	content, ok := last["content"].(string)
	require.True(t, ok)

	return content
}

// TestPlanPrefixesTheUserMessageWithTodaysDateFromThePinnedClock mirrors
// toolcall's own test of the same name.
func TestPlanPrefixesTheUserMessageWithTodaysDateFromThePinnedClock(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, time.September, 16, 9, 0, 0, 0, time.UTC) }
	fixture := newPlannerWithClock(t, fixtureCatalog(), clock, chatContent(t, `{"kind":"none"}`))

	_, err := fixture.planner.Plan(
		context.Background(), "在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, "今日は 2026-09-16（水）です。\n\n在庫を見せて", userContent(t, fixture.requests[0]))
}

// TestPlanDateLineWeekdayForTwoDifferentDates mirrors toolcall's own test
// of the same name: 2026-09-16 is a Wednesday, 2026-09-20 a Sunday.
func TestPlanDateLineWeekdayForTwoDifferentDates(t *testing.T) {
	cases := []struct {
		date time.Time
		want string
	}{
		{time.Date(2026, time.September, 16, 0, 0, 0, 0, time.UTC), "今日は 2026-09-16（水）です。\n\n在庫を見せて"},
		{time.Date(2026, time.September, 20, 0, 0, 0, 0, time.UTC), "今日は 2026-09-20（日）です。\n\n在庫を見せて"},
	}

	for _, tc := range cases {
		date := tc.date
		fixture := newPlannerWithClock(t, fixtureCatalog(), func() time.Time { return date }, chatContent(t, `{"kind":"none"}`))

		_, err := fixture.planner.Plan(
			context.Background(), "在庫を見せて", nil, nil,
			usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil,
		)
		require.NoError(t, err)

		assert.Equal(t, tc.want, userContent(t, fixture.requests[0]))
	}
}
