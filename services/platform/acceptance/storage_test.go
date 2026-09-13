package acceptance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// concurrentWorkspaceCreations is the measurement docs/specs/storage.md's
// section 1 describes: forty concurrent workspace creations against a
// platform started exactly the way the browser gates start one. Ten is not
// enough to reproduce the roughly ten percent failure rate that measurement
// found; forty is the number the spec used.
const concurrentWorkspaceCreations = 40

// TestConcurrentSessionReadsAndWritesAllSucceed is AC-S-101: forty
// concurrent requests, each of which reads the caller's session
// (requireSession runs on every request, before the handler itself) and
// then writes a row (creating a workspace), must all succeed.
//
// Before docs/specs/storage.md's S1-S3 this failed about ten percent of the
// time with "database is locked (5) (SQLITE_BUSY)" - not in the write, in
// the session read every request does first.
func TestConcurrentSessionReadsAndWritesAllSucceed(t *testing.T) {
	server := newTestApp(t, &app.Config{})

	var wg sync.WaitGroup

	statuses := make([]int, concurrentWorkspaceCreations)
	errs := make([]error, concurrentWorkspaceCreations)

	for i := range concurrentWorkspaceCreations {
		wg.Go(func() {
			statuses[i], errs[i] = postWorkspace(t, server)
		})
	}

	wg.Wait()

	failures := 0

	for i := range concurrentWorkspaceCreations {
		if errs[i] != nil {
			failures++

			t.Logf("request %d: %v", i, errs[i])

			continue
		}

		if statuses[i] != http.StatusCreated {
			failures++

			t.Logf("request %d: status %d, want %d", i, statuses[i], http.StatusCreated)
		}
	}

	assert.Zero(t, failures, "%d of %d concurrent workspace creations failed", failures, concurrentWorkspaceCreations)
}

// postWorkspace sends one POST /api/workspaces - the same request
// workspace_test.go's doJSON sends - but returns its outcome instead of
// calling require: t.FailNow, which both require and assert call on
// failure, may only run on the goroutine executing the test function
// (https://pkg.go.dev/testing#T.FailNow), and this is called from forty
// goroutines that are not it.
func postWorkspace(t *testing.T, server *httptest.Server) (int, error) {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"name": "並行ワークスペース"})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodPost, server.URL+"/api/workspaces", strings.NewReader(string(raw)),
	)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := server.Client().Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode, nil
}
