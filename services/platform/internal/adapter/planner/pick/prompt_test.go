package pick_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
)

// clientTSPath is e2e/narrowing/pick/client.ts, computed from this test
// file's own location (runtime.Caller) rather than a path baked in as a
// literal guess - the plan's own path was off by one directory, caught by
// running this test (see docs/plans/staging.md, Task 1, deviation noted in
// STATE.md/the commit that adds this file).
func clientTSPath(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..", "..", "e2e", "narrowing", "pick", "client.ts")
}

// TestSystemPromptIsByteIdenticalToTheTypeScriptSource is AC-S-103's Go
// half: SystemPrompt must appear, verbatim, inside
// e2e/narrowing/pick/client.ts's own PICK_SYSTEM_PROMPT template literal -
// read from that file, never hand-copied into this test.
func TestSystemPromptIsByteIdenticalToTheTypeScriptSource(t *testing.T) {
	raw, err := os.ReadFile(clientTSPath(t))
	require.NoError(t, err, "read e2e/narrowing/pick/client.ts")

	ts := string(raw)

	require.Contains(t, ts, pick.SystemPrompt,
		"pick.SystemPrompt must appear verbatim inside e2e/narrowing/pick/client.ts's PICK_SYSTEM_PROMPT")
}

// TestCandidateLineFramingMatchesTheTypeScriptSource asserts the
// candidate-line format and the surrounding framing this package's
// userMessage builds (prompt.go, unexported) are the same literals
// candidateLine and userMessageFor build in
// e2e/narrowing/pick/client.ts - checked as substrings of that file's own
// source, since userMessage itself is unexported and this is a
// cross-language byte-identity check, not a call into the TS.
func TestCandidateLineFramingMatchesTheTypeScriptSource(t *testing.T) {
	raw, err := os.ReadFile(clientTSPath(t))
	require.NoError(t, err, "read e2e/narrowing/pick/client.ts")

	ts := string(raw)

	require.Contains(t, ts, "質問: ", "the question framing must match the TypeScript source verbatim")
	require.Contains(t, ts, "\\n\\n候補:\\n", "the candidate-list framing must match the TypeScript source verbatim")
	require.True(t, strings.Contains(ts, "candidate.operationId") && strings.Contains(ts, "candidate.serviceDisplayName") &&
		strings.Contains(ts, "candidate.summary"),
		"the TypeScript candidateLine must still join operationId, serviceDisplayName and summary")
}
