// Package httpserver wires the generated OpenAPI server, request validation
// and, optionally, a static file server into one http.Handler.
package httpserver

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

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
		root.Handle("/", spaHandler(staticDir))
	}

	return root, nil
}

// spaHandler serves the built frontend from dir. A request that resolves
// to a real file under dir - an asset, or "/" itself - gets that file,
// with its own content type, from http.FileServer. Everything else is
// treated as a screen's address rather than a file request: when the path
// has no file extension, the client-side router will resolve it, so the
// response is index.html regardless of what follows the host, which is
// what lets a deep link like /workspaces/abc survive a reload. A path that
// does carry an extension but does not exist - a browser asking for a
// build asset that is not there - still answers 404 instead: handing it
// HTML would fail to parse as the script or stylesheet it asked for
// (docs/specs/routing.md section 4, AC-R-102).
//
// Traversal: http.Dir.Open - used here to test whether a file exists, and
// by http.FileServer to serve it - runs path.Clean("/"+name) before
// joining the result onto dir, so "/../etc/passwd" cleans to
// "/etc/passwd" under dir rather than escaping it; the index.html fallback
// never touches the request path at all, only a name this function
// chooses. net/http's own ServeMux also collapses ".." segments in the
// request URL before routing to this handler. Checked with both a plain
// and a percent-encoded traversal attempt (router_test.go).
func spaHandler(dir string) http.Handler {
	root := http.Dir(dir)
	fileServer := http.FileServer(root)
	indexPath := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
		}

		if f, err := root.Open(upath); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		if path.Ext(upath) != "" {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
}
