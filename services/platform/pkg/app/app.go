// Package app is the platform's composition root: the only public package,
// and the only place that wires the object graph together.
package app

import (
	"fmt"
	"net/http"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
)

// New builds the platform's http.Handler. staticDir, when non-empty, is
// served at "/" as the built frontend. Acceptance tests and cmd/api both go
// through this one function, so both exercise the same object graph; neither
// needs to see internal/, which is the point of pkg/app being the only
// public package.
func New(staticDir string) (http.Handler, error) {
	return build(staticDir, httpserver.NewRouter)
}

// build takes the router constructor as a parameter so that the failure path
// - which httpserver.NewRouter cannot exercise with a valid, embedded spec -
// stays under test here.
func build(
	staticDir string,
	newRouter func(openapi.StrictServerInterface, string) (http.Handler, error),
) (http.Handler, error) {
	router, err := newRouter(handler.NewHealth(), staticDir)
	if err != nil {
		return nil, fmt.Errorf("building the router: %w", err)
	}

	return router, nil
}
