package app_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

// The full-catalogue Jev trial's own service router (docs/measurements/
// jev-full-catalogue.md; DECISIONS.md 2026-09-18) app-wiring tests, split
// out the same way the jev picker's own live in app_picker_test.go.

// TestNewRejectsAnUnknownServiceRouter mirrors TestNewRejectsAnUnknownPicker:
// Config.ServiceRouter.Name is the same defence-in-depth as
// Config.Picker.Name - cmd/api never reaches this path because
// internal/infra/config.Load already validates ORCHESTRA_SERVICE_ROUTER,
// but a caller that builds a Config directly gets the same refusal.
func TestNewRejectsAnUnknownServiceRouter(t *testing.T) {
	_, err := app.New(&app.Config{
		ServiceRouter: app.ServiceRouter{Name: "not-a-real-router"},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrInvalidServiceRouter)
}

// TestNewRejectsServiceRouterJevWithoutAnAPIKey mirrors
// TestNewRejectsPickerJevWithoutAnAPIKey: a Config built directly with
// ServiceRouter.Name "jev" and no JevAPIKey gets the same refusal
// config.Load's own ErrMissingJevAPIKey already gives
// ORCHESTRA_SERVICE_ROUTER=jev with no ORCHESTRA_JEV_API_KEY.
func TestNewRejectsServiceRouterJevWithoutAnAPIKey(t *testing.T) {
	_, err := app.New(&app.Config{
		ServiceRouter: app.ServiceRouter{Name: app.ServiceRouterJev},
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, app.ErrMissingJevAPIKey)
}

// TestNewWithNoServiceRouterBuildsUnchanged proves ServiceRouter's zero
// value ("" == ServiceRouterNone) builds exactly as every test in this
// package that predates this subproject already does - the default-off
// guarantee this file's own DECISIONS.md entry asks for, one level up
// from usecase.TestPlanWithNoServiceRouterOffersByteIdenticalTools.
func TestNewWithNoServiceRouterBuildsUnchanged(t *testing.T) {
	_, err := app.New(&app.Config{
		DBPath:        filepath.Join(t.TempDir(), "app.db"),
		AdminPassword: appTestAdminPassword,
	})

	require.NoError(t, err)
}
