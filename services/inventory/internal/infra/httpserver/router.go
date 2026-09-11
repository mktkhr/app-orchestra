// Package httpserver wires the generated OpenAPI server and request
// validation into one http.Handler.
package httpserver

import (
	"fmt"
	"net/http"

	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/adapter/openapi"
)

// NewRouter builds the inventory service's http.Handler: every request is
// validated against the embedded OpenAPI spec before it reaches si.
func NewRouter(si openapi.StrictServerInterface) (http.Handler, error) {
	spec, err := openapi.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("loading embedded OpenAPI spec: %w", err)
	}

	// The spec carries a relative server ("/"); validation only needs the paths.
	spec.Servers = nil

	validator := nethttpmiddleware.OapiRequestValidator(spec)

	mux := http.NewServeMux()

	return validator(openapi.HandlerFromMux(openapi.NewStrictHandler(si, nil), mux)), nil
}
