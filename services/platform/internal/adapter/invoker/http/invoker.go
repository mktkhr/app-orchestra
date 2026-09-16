// Package http implements usecase.Invoker by calling one operation of one
// configured service over HTTP, using the shape (method, path, parameters,
// request body) reflected into the domain.Endpoint by
// internal/adapter/specsource/http.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// bodyPropertyName is the input-schema property usecase.ToolsFor uses to
// carry a non-object request body whole (see mergeRequestBody in
// internal/usecase/tools.go); the invoker must recognise the same name to
// route it back into the request body rather than the query string.
const bodyPropertyName = "body"

// maxErrorDetailBytes bounds how much of a failing response body is read
// into an error message: enough to be useful, small enough not to log a
// service's entire (possibly huge) error page.
const maxErrorDetailBytes = 4096

// ErrUnknownService is wrapped into the error returned when an endpoint
// names a service the Invoker was not configured with.
var ErrUnknownService = errors.New("unknown service")

// ErrServiceError is wrapped into the error returned when a service
// answers a call with a non-2xx status.
var ErrServiceError = errors.New("service returned an error")

// Service is one configured service: a name and the base URL its
// operations are called against. It mirrors config.Service and
// specsourcehttp.Service so this package does not depend on internal/infra
// (an adapter package may not depend on infra, see
// harness/quality/go/golangci.yml).
type Service struct {
	Name string
	URL  string
}

// Invoker calls one endpoint of one configured service and decodes its
// JSON response. It implements usecase.Invoker.
type Invoker struct {
	baseURLs map[string]string
	client   *http.Client
}

var _ usecase.Invoker = (*Invoker)(nil)

// New builds an Invoker for the given services. A nil client defaults to
// http.DefaultClient.
func New(services []Service, client *http.Client) *Invoker {
	if client == nil {
		client = http.DefaultClient
	}

	baseURLs := make(map[string]string, len(services))
	for _, svc := range services {
		baseURLs[svc.Name] = strings.TrimSuffix(svc.URL, "/")
	}

	return &Invoker{baseURLs: baseURLs, client: client}
}

// Invoke calls e with args, splitting them across e's path parameters,
// query parameters and request body per its declared shape, and returns
// the decoded JSON response.
//
// e is a pointer, not the value shown in the usecase.Invoker port, because
// domain.Endpoint is 136 bytes: golangci-lint's gocritic hugeParam check
// (part of the fixed harness policy, see harness/quality/go/golangci.yml)
// rejects passing it by value - the same reason domain.Render and
// Endpoint.IsSafe take a pointer.
func (i *Invoker) Invoke(ctx context.Context, e *domain.Endpoint, args map[string]any) (any, error) {
	baseURL, ok := i.baseURLs[e.Service]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownService, e.Service)
	}

	parts := splitArgs(e, args)

	reqURL := baseURL + parts.path
	if len(parts.query) > 0 {
		reqURL += "?" + parts.query.Encode()
	}

	bodyReader, contentType, err := encodeBody(parts.body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, e.Method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("building request for %s %s: %w", e.Method, reqURL, err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return i.do(req)
}

// do sends req and decodes a successful JSON response.
func (i *Invoker) do(req *http.Request) (any, error) {
	resp, err := i.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s %s: %w", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, serviceError(req, resp)
	}

	if resp.ContentLength == 0 {
		return map[string]any{}, nil
	}

	var data any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding response from %s %s: %w", req.Method, req.URL, err)
	}

	return data, nil
}

// serviceError builds the error for a non-2xx response, folding in as much
// of the body as maxErrorDetailBytes allows. A failure to read that body is
// not itself fatal: the status code alone is still worth reporting.
//
// It wraps two things at once: ErrServiceError, which every caller already
// checked for before this fix, and a usecase.ServiceError carrying the
// status and the body's own message (see serviceMessage) - what
// Orchestrator.invokeAndRender needs to tell a service saying "no" (4xx)
// apart from a platform or service fault (5xx and worse), without parsing
// this error's string. Go's multi-%w (since 1.20) puts both in the chain,
// so errors.Is(err, ErrServiceError) and errors.As(err, &usecase.ServiceError{})
// both still see it.
func serviceError(req *http.Request, resp *http.Response) error {
	detail, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorDetailBytes))
	if readErr != nil {
		detail = nil
	}

	svcErr := usecase.ServiceError{Status: resp.StatusCode, Message: serviceMessage(detail)}

	return fmt.Errorf("%w: %w: %s %s -> %d: %s",
		ErrServiceError, svcErr, req.Method, req.URL, resp.StatusCode, detail)
}

// serviceMessage reads a failing response body as {"message": "..."} and
// returns its message, or "" when the body is empty, is not JSON, or has
// no such field - a service is not required to answer its errors this
// way, and one that does not still gets a usecase.ServiceError, just with
// no message of its own to carry.
func serviceMessage(detail []byte) string {
	var body struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(detail, &body); err != nil {
		return ""
	}

	return body.Message
}

// encodeBody marshals a non-nil body as JSON, returning the reader to send
// and the Content-Type header to set. A nil body needs neither.
func encodeBody(body map[string]any) (io.Reader, string, error) {
	if body == nil {
		return http.NoBody, "", nil
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("encoding request body: %w", err)
	}

	return bytes.NewReader(encoded), "application/json", nil
}

// requestParts is args, partitioned across an endpoint's path, query
// string and request body. A struct rather than three return values: this
// codebase's lint policy requires unnamed results (nonamedreturns) but
// also names for multiple results of ambiguous purpose (gocritic
// unnamedResult, part of the fixed harness policy, see
// harness/quality/go/golangci.yml) - a struct satisfies both at once.
type requestParts struct {
	path  string
	query url.Values
	body  map[string]any
}

// splitArgs partitions args across e's path, query and body, per its
// declared parameters and request body: a name matching a path parameter
// substitutes into the path; a name matching a query parameter is added to
// the query string; everything else, when e has a request body, becomes a
// body field (or, for a non-object body, the single "body" property
// usecase.ToolsFor uses to carry it - see bodyPropertyName above).
func splitArgs(e *domain.Endpoint, args map[string]any) requestParts {
	parts := requestParts{path: e.Path, query: url.Values{}}

	byName := make(map[string]*domain.Parameter, len(e.Parameters))
	for i := range e.Parameters {
		byName[e.Parameters[i].Name] = &e.Parameters[i]
	}

	for name, value := range args {
		if value == nil {
			continue
		}

		if p, ok := byName[name]; ok {
			applyParameter(&parts.path, parts.query, p, value)

			continue
		}

		if e.RequestBody == nil {
			continue
		}

		if parts.body == nil {
			parts.body = map[string]any{}
		}

		assignBodyField(parts.body, e.RequestBody, name, value)
	}

	return parts
}

// applyParameter routes one argument into the path or the query string,
// per the parameter's declared location.
func applyParameter(path *string, query url.Values, p *domain.Parameter, value any) {
	if p.In == "path" {
		*path = strings.ReplaceAll(*path, "{"+p.Name+"}", fmt.Sprint(value))

		return
	}

	query.Set(p.Name, fmt.Sprint(value))
}

// assignBodyField adds one argument to the request body: directly, when
// the body is an object (its fields are just more arguments from the
// model's point of view - see mergeRequestBody in internal/usecase/tools.go),
// or under bodyPropertyName, when a non-object body carries the whole
// payload as that single property.
func assignBodyField(body map[string]any, requestBody *domain.Schema, name string, value any) {
	if requestBody.Type == domain.SchemaTypeObject {
		body[name] = value

		return
	}

	if name == bodyPropertyName {
		body[bodyPropertyName] = value
	}
}
