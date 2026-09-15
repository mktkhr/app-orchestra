// Package llamaswap implements usecase.Narrower against llama-swap's
// /v1/embeddings and /v1/rerank endpoints (docs/specs/shortlisting.md,
// section 2 - H1, H3, H4): Load embeds the whole catalogue once, at
// startup, and holds the result in memory as normalised vectors; Narrow
// embeds only the question, retrieves the best retrieveCount endpoints by
// cosine similarity and reranks exactly those, keeping the reranker's own
// order (H1's "cut to 20, in reranker order").
//
// Matches internal/adapter/planner/chat's HTTP conventions: one
// *http.Client with a bounded timeout, a wrapped sentinel error on a
// non-2xx response, and the caller (this package's own tests) supplying a
// fake transport rather than a real llama-swap - make check must call no
// model (docs/specs/shortlisting.md, AC-H-105).
package llamaswap

import "github.com/mktkhr/app-orchestra/services/platform/internal/domain"

// documentPrefix and queryPrefix are e5-large-q8's own contract
// (e2e/narrowing/embedding/configs.ts's "e5-large-q8" row): the exact
// prefixes docs/specs/shortlisting.md's measurement used, so the product
// retrieves against what was measured rather than a plausible-looking
// guess at the same model's contract.
const (
	documentPrefix = "passage: "
	queryPrefix    = "query: "
)

// retrieveCount is how many candidates the embedding stage hands the
// reranker (docs/plans/shortlisting.md, Task 1 Step 3(b)) - the same 50
// e2e/narrowing/rerank/narrower.ts measured as enough room for an answer
// to land in even when the embedder alone does not rank it first.
const retrieveCount = 50

// endpointID identifies one entry's endpoint the way domain.Catalog.Find
// does: (service, operation id) is the catalogue's own key, and Narrow
// uses it to keep only the entries that are still present in the
// permission-narrowed catalogue it is called with on any given request
// (internal/usecase/orchestrator.go's catalogFor runs before Narrow).
type endpointID struct {
	Service     string
	OperationID string
}

// entry is one endpoint Load embedded: its document text (combinedTextOf,
// below), the endpoint itself (returned by Narrow), its own normalised
// vector, and one normalised vector per example, in Examples' own order.
type entry struct {
	id       endpointID
	endpoint domain.Endpoint
	text     string
	vector   []float32
	examples []string
	// exampleVectors holds one normalised vector per examples entry, in
	// the same order - nil, not a zero-length slice with nothing in it,
	// for an endpoint whose contract declares none (Step 3(c): "an
	// endpoint whose examples are empty still works").
	exampleVectors [][]float32
}

// combinedTextOf is the document text an endpoint is embedded and
// reranked against: summary, description, display name and service
// display name, joined by newlines, in that order - identical to
// e2e/narrowing/lexical.ts's own combinedTextOf, field for field and join
// for join (docs/plans/shortlisting.md, Task 1 Step 4), so the product
// retrieves against exactly what was measured. A real service's own
// `also` terms (docs/specs/shortlisting.md, H1) live in description, not
// summary - dropping it would silently change what retrieval reads.
func combinedTextOf(e *domain.Endpoint) string {
	return e.Summary + "\n" + e.Description + "\n" + e.DisplayName + "\n" + e.ServiceDisplayName
}

// idOf is endpointID's constructor from an *domain.Endpoint, named once so
// index.go, embed.go and rerank.go all build the same key the same way.
func idOf(e *domain.Endpoint) endpointID {
	return endpointID{Service: e.Service, OperationID: e.OperationID}
}
