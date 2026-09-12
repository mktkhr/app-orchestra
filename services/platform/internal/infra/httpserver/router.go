// Package httpserver wires the generated OpenAPI server, request validation
// and, optionally, a static file server into one http.Handler.
package httpserver

import (
	"fmt"
	"net/http"

	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

// NewRouter builds the platform's http.Handler: every request is validated
// against the embedded OpenAPI spec, and resolved to a signed-in user (or
// refused with 401 - requireSession), before it reaches si. When staticDir
// is non-empty, anything that is not "/api/..." is served from that
// directory, unauthenticated, so the built frontend - the sign-in screen
// included - and the API share one origin (docs/plans/auth.md, Task 2:
// static files carry no cookie to check, so there is nothing here for
// requireSession to gate).
//
// sessions must never be nil in production - see requireSession's own doc
// comment for why pkg/app.New now guarantees that.
func NewRouter(si openapi.StrictServerInterface, staticDir string, sessions SessionUsers) (http.Handler, error) {
	spec, err := openapi.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("loading embedded OpenAPI spec: %w", err)
	}

	// The spec carries relative servers ("/"); validation only needs the paths.
	spec.Servers = nil

	validator := nethttpmiddleware.OapiRequestValidator(spec)

	apiMux := http.NewServeMux()
	apiHandler := validator(openapi.HandlerFromMux(openapi.NewStrictHandler(si, nil), apiMux))
	apiHandler = requireSession(sessions, apiHandler)

	root := http.NewServeMux()
	root.Handle("/api/", apiHandler)

	if staticDir != "" {
		root.Handle("/", http.FileServer(http.Dir(staticDir)))
	}

	return root, nil
}
