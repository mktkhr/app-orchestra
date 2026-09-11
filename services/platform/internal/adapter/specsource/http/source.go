// Package http implements usecase.SpecSource by fetching each configured
// service's own OpenAPI contract over HTTP and converting it into a
// domain.Catalog.
package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// specPath is appended to every service's configured base URL.
const specPath = "/openapi.yaml"

// ErrUnexpectedStatus is wrapped into the error returned when a service
// answers /openapi.yaml with anything but 200.
var ErrUnexpectedStatus = errors.New("unexpected status fetching openapi spec")

// Service is one configured service: a name and the base URL its
// /openapi.yaml is fetched from. It mirrors config.Service so this package
// does not depend on internal/infra/config (an adapter package may not
// depend on infra, see harness/quality/go/golangci.yml).
type Service struct {
	Name string
	URL  string
}

// Source fetches the OpenAPI contract of each configured service over HTTP
// and converts every operation into a domain.Endpoint. It implements
// usecase.SpecSource.
type Source struct {
	services []Service
	client   *http.Client
}

var _ usecase.SpecSource = (*Source)(nil)

// New builds a Source for the given services. A nil client defaults to
// http.DefaultClient.
func New(services []Service, client *http.Client) *Source {
	if client == nil {
		client = http.DefaultClient
	}

	return &Source{services: services, client: client}
}

// Fetch gets every configured service's /openapi.yaml and converts it into
// one domain.Catalog. A service that cannot be reached, or whose spec
// cannot be parsed, fails the whole fetch: a partial catalogue would
// silently hide endpoints that exist, which is worse than failing loudly.
func (s *Source) Fetch(ctx context.Context) (domain.Catalog, error) {
	var catalog domain.Catalog

	for _, svc := range s.services {
		endpoints, err := s.fetchOne(ctx, svc)
		if err != nil {
			return domain.Catalog{}, fmt.Errorf("fetching %s catalogue: %w", svc.Name, err)
		}

		catalog.Endpoints = append(catalog.Endpoints, endpoints...)
	}

	return catalog, nil
}

// fetchOne GETs one service's spec and parses it.
func (s *Source) fetchOne(ctx context.Context, svc Service) ([]domain.Endpoint, error) {
	url := strings.TrimSuffix(svc.URL, "/") + specPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", url, err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %w (status %d)", url, ErrUnexpectedStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	endpoints, err := parseSpec(svc.Name, body)
	if err != nil {
		return nil, fmt.Errorf("parsing spec from %s: %w", url, err)
	}

	return endpoints, nil
}
