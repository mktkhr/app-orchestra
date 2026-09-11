// Package app is the attendance service's composition root: the only public
// package, and the only place that wires the object graph together.
package app

import (
	"fmt"
	"net/http"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/adapter/repository"
	"github.com/mktkhr/app-orchestra/services/attendance/internal/infra/httpserver"
)

// New builds the attendance service's http.Handler. Acceptance tests and
// cmd/api both go through this one function, so both exercise the same
// object graph; neither needs to see internal/, which is the point of
// pkg/app being the only public package.
func New() (http.Handler, error) {
	return build(httpserver.NewRouter)
}

// build takes the router constructor as a parameter so that the failure path
// - which httpserver.NewRouter cannot exercise with a valid, embedded spec -
// stays under test here.
func build(
	newRouter func(openapi.StrictServerInterface) (http.Handler, error),
) (http.Handler, error) {
	store := repository.NewMemory()

	router, err := newRouter(handler.NewRecords(store))
	if err != nil {
		return nil, fmt.Errorf("building the router: %w", err)
	}

	return router, nil
}
