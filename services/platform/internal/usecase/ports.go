// Package usecase holds the platform's ports and orchestration logic: the
// interfaces the adapters implement, and (in later tasks) the code that
// drives them. Nothing here performs I/O directly; it depends on domain and
// nothing else.
package usecase

import (
	"context"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// SpecSource fetches the OpenAPI contract of every configured service and
// converts it into the platform's catalogue. Implemented by
// internal/adapter/specsource/http, which reads each service's
// /openapi.yaml over HTTP; kept as a port here so the usecase layer depends
// on the shape of the fetch, not on HTTP.
//
// A service that cannot be reached must fail the whole fetch: a partial
// catalogue would silently hide endpoints that exist, which is worse than
// failing loudly at startup.
type SpecSource interface {
	Fetch(ctx context.Context) (domain.Catalog, error)
}
