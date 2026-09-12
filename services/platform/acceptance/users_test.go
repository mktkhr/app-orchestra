package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// accountOnWire is the wire shape of one User, as GET /api/users lists it.
type accountOnWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// permissionOnWire is the wire shape of one Permission.
type permissionOnWire struct {
	Service     string `json:"service"`
	OperationID string `json:"operationId"`
}

// nonAdminName is the account docs/plans/auth.md, Task 3's own tests need
// but have no HTTP route to create (docs/specs/auth.md, section 8: no
// account creation through the UI, still). It is seeded through
// app.Config.SeedAccounts - pkg/app's own composition-root seam
// (pkg/app/app.go's seedAccounts, wired the same way AdminPassword itself
// seeds the first admin) - never through a network-reachable "anyone can
// register" endpoint: nothing in cmd/api or internal/infra/config ever
// sets SeedAccounts, so this seam only exists for a Go caller of
// pkg/app.New, which an end user is not.
const nonAdminName = "yamada"
const nonAdminPassword = "correct horse battery staple 2"

// newClientWithJar builds an *http.Client with its own cookie jar, so a
// second person's session (seeded by newUsersTestApp, signed in
// separately from the shared server.Client() newTestApp's own admin
// session lives on) never collides with the admin's.
func newClientWithJar(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	return &http.Client{Jar: jar}
}

// doJSONWithClient is doJSON (helpers_test.go), generalised to an explicit
// client and base URL rather than server.Client() - so a request can be
// sent as somebody other than whoever is signed in on the shared server
// client.
func doJSONWithClient(t *testing.T, client *http.Client, baseURL, method, path string, body, out any) int {
	t.Helper()

	var reader *bytes.Reader

	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)

		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, baseURL+path, reader)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	if out != nil {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}

	return resp.StatusCode
}

// signInAs signs client in as name/password against server, so every
// later request through client carries that person's session cookie.
func signInAs(t *testing.T, client *http.Client, server *httptest.Server, name, password string) {
	t.Helper()

	status := doJSONWithClient(t, client, server.URL, http.MethodPost, "/api/session", map[string]any{
		"name":     name,
		"password": password,
	}, nil)
	require.Equal(t, http.StatusOK, status, "signing in as %s", name)
}

// newUsersTestApp builds a platform with inventory the only configured
// service, admin (adminPassword, helpers_test.go) and one non-admin
// account (nonAdminName/nonAdminPassword, seeded via SeedAccounts) both
// already accounts of the platform - newTestApp itself has already signed
// in as the admin on server.Client() by the time this returns.
func newUsersTestApp(t *testing.T) (*httptest.Server, *fixtureService) {
	t.Helper()

	inventory := newFixtureService(t, inventorySpec, "/api/inventory/items", `{"items":[{"id":"1"}]}`)

	server := newTestApp(t, &app.Config{
		Services: []app.Service{{Name: "inventory", URL: inventory.server.URL}},
		SeedAccounts: []app.SeedAccount{
			{Name: nonAdminName, Password: nonAdminPassword, Role: "user"},
		},
		PlanFixtures: []app.PlanFixture{
			{Query: "在庫の一覧を見せて", Service: "inventory", OperationID: "ListInventoryItems"},
		},
	})

	return server, inventory
}

// findAccountID returns the id of the account named name among accounts.
func findAccountID(t *testing.T, accounts []accountOnWire, name string) string {
	t.Helper()

	for _, a := range accounts {
		if a.Name == name {
			return a.ID
		}
	}

	t.Fatalf("no account named %q among %+v", name, accounts)

	return ""
}

// TestAdminReadsAccountsGrantsPermissionsAndThePersonsNextQuestionUsesThem
// is docs/plans/auth.md, Task 3, Step 2's main scenario: an admin reads
// the accounts and sets another person's permissions, and that person's
// next question is answered from the service they were just granted -
// proof the grant actually reached the catalogue (docs/specs/auth.md,
// section 5), not just the permissions table.
func TestAdminReadsAccountsGrantsPermissionsAndThePersonsNextQuestionUsesThem(t *testing.T) {
	server, inventory := newUsersTestApp(t)

	// The admin reads every account (AC-A-106).
	var accounts []accountOnWire
	status := doJSON(t, server, http.MethodGet, "/api/users", nil, &accounts)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, accounts, 2)

	yamadaID := findAccountID(t, accounts, nonAdminName)

	yamada := newClientWithJar(t)
	signInAs(t, yamada, server, nonAdminName, nonAdminPassword)

	// Before any grant, yamada may call nothing: the same operation an
	// admin can invoke freely is refused as not found - the same status a
	// genuinely unknown one gets (AC-A-104, at the platform level).
	status = doJSONWithClient(t, yamada, server.URL, http.MethodPost, "/api/invoke", map[string]any{
		"service":     "inventory",
		"operationId": "ListInventoryItems",
		"args":        map[string]any{},
	}, nil)
	assert.Equal(t, http.StatusBadRequest, status, "no permission granted yet")

	// The admin reads yamada's permissions: empty, nothing granted yet.
	var permissions []permissionOnWire
	status = doJSON(t, server, http.MethodGet, "/api/users/"+yamadaID+"/permissions", nil, &permissions)
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, permissions)

	// The admin grants yamada the one inventory operation.
	status = doJSON(t, server, http.MethodPut, "/api/users/"+yamadaID+"/permissions", map[string]any{
		"permissions": []map[string]any{
			{"service": "inventory", "operationId": "ListInventoryItems"},
		},
	}, nil)
	require.Equal(t, http.StatusNoContent, status)

	// Reading it back shows exactly the one row just granted.
	status = doJSON(t, server, http.MethodGet, "/api/users/"+yamadaID+"/permissions", nil, &permissions)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, permissions, 1)
	assert.Equal(t, "inventory", permissions[0].Service)
	assert.Equal(t, "ListInventoryItems", permissions[0].OperationID)

	// yamada's next question is answered from the service just granted -
	// the catalogue, not only the permissions table, now reflects the
	// grant (docs/specs/auth.md, section 5: the tool list and /api/invoke
	// read the same narrowed catalogue).
	var plan planResponse
	status = doJSONWithClient(t, yamada, server.URL, http.MethodPost, "/api/plan", map[string]any{
		"query": "在庫の一覧を見せて",
	}, &plan)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "result", plan.Kind)
	assert.Equal(t, "inventory", plan.Source.Service)
	assert.Equal(t, "ListInventoryItems", plan.Source.OperationID)
	assert.NotEmpty(t, inventory.requests, "the grant must have reached the inventory service itself")
}

// TestNonAdminGetsForbiddenFromEveryUsersEndpoint is docs/plans/auth.md,
// Task 3, Step 2's other half: a non-admin gets 403, not 401 - they are
// signed in, just not allowed - from all three admin endpoints
// (AC-A-106).
func TestNonAdminGetsForbiddenFromEveryUsersEndpoint(t *testing.T) {
	server, _ := newUsersTestApp(t)

	var accounts []accountOnWire
	status := doJSON(t, server, http.MethodGet, "/api/users", nil, &accounts)
	require.Equal(t, http.StatusOK, status)
	adminID := findAccountID(t, accounts, "admin")

	yamada := newClientWithJar(t)
	signInAs(t, yamada, server, nonAdminName, nonAdminPassword)

	status = doJSONWithClient(t, yamada, server.URL, http.MethodGet, "/api/users", nil, nil)
	assert.Equal(t, http.StatusForbidden, status, "GET /api/users")

	status = doJSONWithClient(t, yamada, server.URL, http.MethodGet, "/api/users/"+adminID+"/permissions", nil, nil)
	assert.Equal(t, http.StatusForbidden, status, "GET /api/users/{id}/permissions")

	status = doJSONWithClient(t, yamada, server.URL, http.MethodPut, "/api/users/"+adminID+"/permissions", map[string]any{
		"permissions": []map[string]any{},
	}, nil)
	assert.Equal(t, http.StatusForbidden, status, "PUT /api/users/{id}/permissions")
}

// TestUsersEndpointsAre401WithoutASession is AC-A-102 for this file's own
// three routes: unlike the 403 above, nobody is signed in at all here.
func TestUsersEndpointsAre401WithoutASession(t *testing.T) {
	server, _ := newUsersTestApp(t)

	anon := newClientWithJar(t)

	status := doJSONWithClient(t, anon, server.URL, http.MethodGet, "/api/users", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, status)

	status = doJSONWithClient(t, anon, server.URL, http.MethodGet, "/api/users/whoever/permissions", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, status)
}
