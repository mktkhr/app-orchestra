package toolcall_test

// Split from planner_test.go: toolcall.WithClock's own tests (TODO.md's
// "invented form values" item, DECISIONS.md 2026-09-16 "Thirty questions
// against the real dev services") - the model is told today's date so it
// stops inventing one (2023-10-10 for a question asked against a
// 2026-09-16 clock). fixtureCatalog, callResponse and captureBody are
// defined in planner_test.go, same package.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// userContentOf extracts the one user message's content from a captured
// request's decoded body - the same shape TestPlanSendsAnswersAlongsideTheQuery
// (planner_test.go) already reads by hand, factored out here since every
// test below needs it.
func userContentOf(t *testing.T, decoded map[string]any) string {
	t.Helper()

	messages, ok := decoded["messages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, messages)

	last, ok := messages[len(messages)-1].(map[string]any)
	require.True(t, ok)

	content, ok := last["content"].(string)
	require.True(t, ok)

	return content
}

// TestPlanPrefixesTheUserMessageWithTodaysDateFromThePinnedClock is the
// date line's own basic case: with no turns and no answers, the user
// message still starts with "今日は <date>（<weekday>）です。" followed by
// a blank line, ahead of the question itself.
func TestPlanPrefixesTheUserMessageWithTodaysDateFromThePinnedClock(t *testing.T) {
	server, requests := captureBody(t)
	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})

	clock := func() time.Time { return time.Date(2026, time.September, 16, 9, 0, 0, 0, time.UTC) }
	planner := toolcall.New(client, fixtureCatalog(), toolcall.WithClock(clock))

	_, err := planner.Plan(
		context.Background(), "在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	content := userContentOf(t, (*requests)[0].decoded)
	assert.Equal(t, "今日は 2026-09-16（水）です。\n\n在庫を見せて", content)
}

// TestPlanDateLineWeekdayForTwoDifferentDates pins two different clocks and
// checks each renders its own correct Japanese weekday - 2026-09-16 is a
// Wednesday, 2026-09-20 a Sunday - so a future change to japaneseWeekday's
// indexing could not silently land on a weekday that is merely plausible.
func TestPlanDateLineWeekdayForTwoDifferentDates(t *testing.T) {
	cases := []struct {
		date    time.Time
		want    string
		comment string
	}{
		{time.Date(2026, time.September, 16, 0, 0, 0, 0, time.UTC), "今日は 2026-09-16（水）です。\n\n在庫を見せて", "Wednesday"},
		{time.Date(2026, time.September, 20, 0, 0, 0, 0, time.UTC), "今日は 2026-09-20（日）です。\n\n在庫を見せて", "Sunday"},
	}

	for _, tc := range cases {
		server, requests := captureBody(t)
		client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})

		date := tc.date
		planner := toolcall.New(client, fixtureCatalog(), toolcall.WithClock(func() time.Time { return date }))

		_, err := planner.Plan(
			context.Background(), "在庫を見せて", nil, nil,
			usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}), nil,
		)
		require.NoError(t, err)

		content := userContentOf(t, (*requests)[0].decoded)
		assert.Equal(t, tc.want, content, tc.comment)
	}
}

// TestPlanWithNoClockOptionUsesRealTime is the option-less default: New
// still sends a plausible-looking date line (2026-09-16 is well before any
// build of this test could plausibly run) without a WithClock override -
// proving New's own default (time.Now) is wired in, not merely the option.
func TestPlanWithNoClockOptionUsesRealTime(t *testing.T) {
	server, requests := captureBody(t)
	client := chat.New(chat.Config{BaseURL: server.URL, Model: "test-model"})
	planner := toolcall.New(client, fixtureCatalog())

	_, err := planner.Plan(
		context.Background(), "在庫を見せて", nil, nil, usecase.ToolsFor(fixtureCatalog(), usecase.PlanContext{WorkspaceID: "ws-1"}),
		nil,
	)
	require.NoError(t, err)

	content := userContentOf(t, (*requests)[0].decoded)
	assert.Regexp(t, `^今日は 20\d\d-\d\d-\d\d（[月火水木金土日]）です。\n\n在庫を見せて$`, content)
}
