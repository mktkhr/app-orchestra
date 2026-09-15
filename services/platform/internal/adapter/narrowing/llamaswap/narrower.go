package llamaswap

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// requestTimeout bounds one embeddings or rerank call, mirroring
// internal/adapter/planner/chat's own requestTimeout: generous, because a
// local model can be slow on modest hardware, but not unbounded.
const requestTimeout = 120 * time.Second

// Narrower implements usecase.Narrower: Load embeds a whole catalogue
// once and holds it in memory; Narrow scores the loaded entries that are
// still present in the catalogue it is called with, retrieves the best
// retrieveCount by cosine similarity, and reranks exactly those.
type Narrower struct {
	baseURL     string
	embedModel  string
	rerankModel string
	http        *http.Client

	entries []entry
}

var _ usecase.Narrower = (*Narrower)(nil)

// New builds a Narrower against baseURL (ORCHESTRA_LLM_BASE_URL - the same
// llama-swap the tool-calling planner talks to, docs/specs/shortlisting.md
// section 5), embedModel and rerankModel. Load must be called once, before
// the first Narrow, to give it something to score against.
//
// ORCHESTRA_LLM_BASE_URL already carries a trailing "/v1" in every real
// deployment (internal/adapter/planner/chat's own Client appends only
// "/chat/completions", so the operator-set value must already end in
// "/v1" for that to reach the right endpoint - see e2e/eval/run.ts's
// default, "http://localhost:11435/v1"). embed.go and rerank.go append
// "/v1/embeddings" and "/v1/rerank" of their own, which would double that
// suffix against the same value - trimming one trailing "/v1" here (a
// no-op against the bare host this package's own tests pass) is what
// keeps one config variable correct for both callers, found by
// e2e/shortlist/run.ts (docs/plans/shortlisting.md Task 4 Step 8) against
// a real llama-swap, not by any existing httptest fake.
func New(baseURL, embedModel, rerankModel string) *Narrower {
	return &Narrower{
		baseURL:     strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1"),
		embedModel:  embedModel,
		rerankModel: rerankModel,
		http:        &http.Client{Timeout: requestTimeout},
	}
}

// VectorCount is how many vectors Load computed and held: one per
// endpoint plus one per example, the number pkg/app logs alongside the
// load's own duration (docs/plans/shortlisting.md, Task 1 Step 6).
func (n *Narrower) VectorCount() int {
	count := 0

	for i := range n.entries {
		count += 1 + len(n.entries[i].exampleVectors)
	}

	return count
}

// Load embeds every endpoint's own document text and each of its examples
// exactly once, in a single /v1/embeddings call (docs/plans/shortlisting.md,
// Task 1 Step 3(a)), and holds the normalised vectors in memory for every
// later Narrow to score against (H3: "computed when the catalogue is
// loaded and held in memory... The question is embedded per request. No
// vector store.").
func (n *Narrower) Load(ctx context.Context, catalog domain.Catalog) error {
	texts, spans := documentsFor(catalog)

	vectors, err := embed(ctx, n.http, n.baseURL, n.embedModel, documentPrefix, texts)
	if err != nil {
		return fmt.Errorf("embedding catalogue: %w", err)
	}

	entries := make([]entry, len(spans))

	for i := range spans {
		span := &spans[i]
		entries[i] = entry{
			id:       span.id,
			endpoint: span.endpoint,
			text:     span.text,
			examples: span.examples,
			vector:   vectors[span.start],
		}

		if len(span.examples) > 0 {
			entries[i].exampleVectors = vectors[span.start+1 : span.start+1+len(span.examples)]
		}
	}

	n.entries = entries

	return nil
}

// span is documentsFor's own bookkeeping: where one endpoint's own text,
// and its examples right after it, land in the flat texts slice Load
// embeds in a single call.
type span struct {
	id       endpointID
	endpoint domain.Endpoint
	text     string
	examples []string
	start    int
}

// documentsFor lays out catalog as one flat slice of texts - every
// endpoint's own combinedTextOf, immediately followed by each of its
// examples - plus the spans needed to read the matching vectors back out
// once embed returns.
func documentsFor(catalog domain.Catalog) ([]string, []span) {
	texts := make([]string, 0, len(catalog.Endpoints))
	spans := make([]span, 0, len(catalog.Endpoints))

	for i := range catalog.Endpoints {
		e := &catalog.Endpoints[i]
		text := combinedTextOf(e)
		start := len(texts)

		texts = append(texts, text)
		texts = append(texts, e.Examples...)

		spans = append(spans, span{id: idOf(e), endpoint: *e, text: text, examples: e.Examples, start: start})
	}

	return texts, spans
}

// Narrow implements usecase.Narrower (docs/specs/shortlisting.md, H1):
// only the loaded entries still present in catalog are considered - a
// caller always passes the permission-narrowed catalogue for this request
// (internal/usecase/orchestrator.go's catalogFor runs before Narrow), so
// an operation a person may not call is never scored, retrieved or
// reranked into their shortlist. The question is embedded once; each
// remaining candidate is scored as the max cosine over its own vector and
// its examples' (H1); the best retrieveCount go to the reranker in one
// call, whose documents are each candidate's text plus its examples
// (H1: "reranked by bge-reranker-v2-m3 reading both"); the top k, in the
// reranker's own order, come back as a domain.Catalog.
func (n *Narrower) Narrow(ctx context.Context, catalog domain.Catalog, query string, k int) (domain.Catalog, error) {
	if k < 0 {
		k = 0
	}

	candidates := n.entriesIn(catalog)
	if len(candidates) == 0 || k == 0 {
		return domain.Catalog{}, nil
	}

	queryVectors, err := embed(ctx, n.http, n.baseURL, n.embedModel, queryPrefix, []string{query})
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("embedding question: %w", err)
	}

	retrieved := retrieve(queryVectors[0], candidates, retrieveCount)

	ids := make([]endpointID, len(retrieved))
	documents := make([]string, len(retrieved))
	byID := make(map[endpointID]*entry, len(retrieved))

	for i, c := range retrieved {
		ids[i] = c.id
		documents[i] = rerankDocumentFor(c)
		byID[c.id] = c
	}

	reranked, err := rerankCandidates(ctx, n.http, n.baseURL, n.rerankModel, query, ids, documents)
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("reranking candidates: %w", err)
	}

	if k > len(reranked) {
		k = len(reranked)
	}

	endpoints := make([]domain.Endpoint, 0, k)
	for _, r := range reranked[:k] {
		endpoints = append(endpoints, byID[r.id].endpoint)
	}

	return domain.Catalog{Endpoints: endpoints}, nil
}

// entriesIn returns the pointers of n.entries whose id is also present in
// catalog, in n.entries' own order - the permission filter Narrow's own
// doc comment describes.
func (n *Narrower) entriesIn(catalog domain.Catalog) []*entry {
	allowed := make(map[endpointID]struct{}, len(catalog.Endpoints))
	for i := range catalog.Endpoints {
		allowed[idOf(&catalog.Endpoints[i])] = struct{}{}
	}

	candidates := make([]*entry, 0, len(n.entries))

	for i := range n.entries {
		if _, ok := allowed[n.entries[i].id]; ok {
			candidates = append(candidates, &n.entries[i])
		}
	}

	return candidates
}

// scored is one candidate's outcome of the embedding stage: the entry and
// its max-cosine score against the question.
type scored struct {
	entry *entry
	score float32
}

// retrieve scores every candidate against queryVector (dot product of two
// unit vectors is a cosine similarity, both already normalised by embed),
// sorts best first, and returns at most limit of them.
func retrieve(queryVector []float32, candidates []*entry, limit int) []*entry {
	scoredCandidates := make([]scored, len(candidates))
	for i, c := range candidates {
		scoredCandidates[i] = scored{entry: c, score: maxCosine(queryVector, c)}
	}

	sort.Slice(scoredCandidates, func(i, j int) bool { return scoredCandidates[i].score > scoredCandidates[j].score })

	if len(scoredCandidates) > limit {
		scoredCandidates = scoredCandidates[:limit]
	}

	out := make([]*entry, len(scoredCandidates))
	for i, s := range scoredCandidates {
		out[i] = s.entry
	}

	return out
}

// maxCosine is H1's own scoring rule: the best cosine similarity between
// queryVector and either c's own vector or any one of its examples' -
// whichever example (or the operation's own text) reads most like the
// question wins, not their average.
func maxCosine(queryVector []float32, c *entry) float32 {
	best := dot(queryVector, c.vector)

	for _, ev := range c.exampleVectors {
		if d := dot(queryVector, ev); d > best {
			best = d
		}
	}

	return best
}

// dot is a dot product, and - since embed normalises every vector to unit
// length - exactly a cosine similarity.
func dot(a, b []float32) float32 {
	var sum float32

	for i := 0; i < len(a) && i < len(b); i++ {
		sum += a[i] * b[i]
	}

	return sum
}

// rerankDocumentFor is the text the reranker reads for one candidate: its
// own document text, then each of its examples, one per line (H1: "reads
// both") - the same join e2e/narrowing/utterances/reranker-written.ts's
// own documentTextWithExamplesOf performs.
func rerankDocumentFor(c *entry) string {
	if len(c.examples) == 0 {
		return c.text
	}

	return strings.Join(append([]string{c.text}, c.examples...), "\n")
}
