package llamaswap_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/narrowing/llamaswap"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// combinedText mirrors llamaswap's own unexported combinedTextOf, so this
// external test package can predict the exact document text Load and
// Narrow send, without depending on the package's internals.
func combinedText(e *domain.Endpoint) string {
	return e.Summary + "\n" + e.Description + "\n" + e.DisplayName + "\n" + e.ServiceDisplayName
}

// endpoint builds a minimal, exposed domain.Endpoint for these tests: a
// safe GET with an object response (Render never has to look further than
// that here), one shared service ("svc" - nothing in this file cares
// about cross-service behaviour), named operationID/displayName, and
// whatever examples the test wants to give it.
func endpoint(operationID, displayName string, examples ...string) domain.Endpoint {
	return domain.Endpoint{
		Service:     "svc",
		OperationID: operationID,
		Method:      domain.MethodGet,
		Path:        "/" + operationID,
		Summary:     operationID + " summary",
		DisplayName: displayName,
		Examples:    examples,
		Response:    &domain.Schema{Type: domain.SchemaTypeObject},
	}
}

// The wire shapes a fake llama-swap answers with - a local mirror of
// OpenAI's /v1/embeddings and bge-reranker's /v1/rerank response shapes,
// kept here rather than imported from the package under test since both
// are unexported implementation detail there.
type embedWireRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedWireItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type rerankWireRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type rerankWireItem struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// writeJSON encodes v as the response body. Handlers run on their own
// goroutine, not the test's (testifylint's go-require), so a failure to
// encode is reported with assert rather than require - there is nothing
// left to protect downstream of it in the same call anyway.
func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	assert.NoError(t, json.NewEncoder(w).Encode(v))
}

// TestLoadPostsEveryEndpointsTextAndEachExampleOnceWithPassagePrefix is
// Task 1 Step 3(a): Load embeds every operation's own document text and
// each of its examples, exactly once, with the "passage: " prefix
// e5-large-q8 measures against (e2e/narrowing/embedding/configs.ts).
func TestLoadPostsEveryEndpointsTextAndEachExampleOnceWithPassagePrefix(t *testing.T) {
	alpha := endpoint("Alpha", "アルファ")
	alpha.Description = "関連語: 品番、品名"
	bravo := endpoint("Bravo", "", "例1", "例2")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{alpha, bravo}}

	require.Equal(t, "Alpha summary\n関連語: 品番、品名\nアルファ\n", combinedText(&alpha),
		"the document text is summary, description, display name and service display name, newline-joined, in that "+
			"order - identical to e2e/narrowing/lexical.ts's own combinedTextOf")

	textAlpha := combinedText(&alpha)
	textBravo := combinedText(&bravo)

	var calls int
	var gotInputs []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "/v1/embeddings", r.URL.Path) {
			return
		}
		calls++

		var req embedWireRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			return
		}
		gotInputs = req.Input

		data := make([]embedWireItem, len(req.Input))
		for i := range req.Input {
			data[i] = embedWireItem{Index: i, Embedding: []float32{1, 0}}
		}

		writeJSON(t, w, map[string]any{"data": data})
	}))
	defer server.Close()

	n := llamaswap.New(server.URL, "embed-model", "rerank-model")

	require.NoError(t, n.Load(context.Background(), catalog))

	assert.Equal(t, 1, calls, "Load must post every document in a single call")
	assert.Equal(t, []string{
		"passage: " + textAlpha,
		"passage: " + textBravo,
		"passage: 例1",
		"passage: 例2",
	}, gotInputs)
	assert.Equal(t, 4, n.VectorCount(), "one vector per endpoint plus one per example")
}

// fakeNarrowServer bundles narrowServer's own return values: the server
// itself, and the documents its most recent /v1/rerank call was sent -
// one type instead of several unnamed return values (gocritic's
// unnamedResult, harness/quality/go/golangci.yml).
type fakeNarrowServer struct {
	server          *httptest.Server
	rerankDocuments *[]string
}

// narrowServer is a fake llama-swap answering /v1/embeddings by exact
// input text (from responses) and /v1/rerank by calling rerank with the
// documents it was sent, recording those documents for the caller to
// assert on - the seam every Narrow test below configures differently to
// control retrieval and rerank scoring independently.
func narrowServer(
	t *testing.T,
	embedResponses map[string][]float32,
	rerank func(documents []string) []rerankWireItem,
) fakeNarrowServer {
	t.Helper()

	var rerankDocuments []string

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/embeddings":
			var req embedWireRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
				return
			}

			data := make([]embedWireItem, len(req.Input))
			for i, text := range req.Input {
				vector, ok := embedResponses[text]
				if !assert.True(t, ok, "no fake embedding configured for %q", text) {
					return
				}
				data[i] = embedWireItem{Index: i, Embedding: vector}
			}

			writeJSON(t, w, map[string]any{"data": data})
		case "/v1/rerank":
			var req rerankWireRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
				return
			}
			rerankDocuments = req.Documents

			writeJSON(t, w, map[string]any{"results": rerank(req.Documents)})
		default:
			assert.Failf(t, "unexpected request path", "%q", r.URL.Path)
		}
	}))

	return fakeNarrowServer{server: fake, rerankDocuments: &rerankDocuments}
}

// identityRerank answers /v1/rerank with each document's own position as
// its relevance score, descending - i.e. the reranker agrees with
// whatever order the documents arrived in. Used by tests that care about
// the embedding stage's own ranking, not the reranker's.
func identityRerank(documents []string) []rerankWireItem {
	results := make([]rerankWireItem, len(documents))
	for i := range documents {
		results[i] = rerankWireItem{Index: i, RelevanceScore: float64(len(documents) - i)}
	}

	return results
}

// TestNarrowScoresCandidatesByMaxCosineNotSum is Task 1 Step 3(b)'s own
// scoring requirement: an endpoint is ranked by the best cosine similarity
// among its own vector and its examples' - not their sum. Bravo's own
// vector reads nothing like the question, but each of its three examples
// reads half as well as Alpha's own text does; under a (wrong) sum score
// summing every example in, Bravo (1.5) would outrank Alpha (1.0). Under
// the max this task actually specifies, Alpha (1.0) outranks Charlie
// (0.8), which outranks Bravo (0.5) - and that is the order the documents
// must reach the reranker in, each already carrying its own text plus its
// examples (H1).
func TestNarrowScoresCandidatesByMaxCosineNotSum(t *testing.T) {
	alpha := endpoint("Alpha", "")
	charlie := endpoint("Charlie", "")
	bravo := endpoint("Bravo", "", "例1", "例2", "例3")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{alpha, charlie, bravo}}

	textAlpha := combinedText(&alpha)
	textCharlie := combinedText(&charlie)
	textBravo := combinedText(&bravo)

	// cos(query, ·): Alpha 1.0, Charlie 0.8, Bravo's own 0.0 and each of
	// its three examples 0.5 - direction (1, sqrt(3)) normalises to
	// (0.5, 0.866...), which dots to exactly 0.5 against query (1, 0).
	embedResponses := map[string][]float32{
		"query: 質問":               {1, 0},
		"passage: " + textAlpha:   {1, 0},
		"passage: " + textCharlie: {0.8, 0.6},
		"passage: " + textBravo:   {0, 1},
		"passage: 例1":             {1, 1.7320508},
		"passage: 例2":             {1, 1.7320508},
		"passage: 例3":             {1, 1.7320508},
	}

	fake := narrowServer(t, embedResponses, identityRerank)
	defer fake.server.Close()

	n := llamaswap.New(fake.server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), catalog))

	_, err := n.Narrow(context.Background(), catalog, "質問", 3)
	require.NoError(t, err)

	wantBravoDocument := textBravo + "\n例1\n例2\n例3"
	assert.Equal(t, []string{textAlpha, textCharlie, wantBravoDocument}, *fake.rerankDocuments,
		"the reranker must see the candidates best-first by max cosine (Alpha, Charlie, Bravo), each with its examples appended")
}

// TestNarrowSortsAnUnsortedRerankResponseAndMapsIndexesBackToEndpoints is
// Task 1 Step 3(b)'s other half: /v1/rerank does not promise its results
// arrive sorted, or in the order the documents were sent - this fake
// deliberately answers out of order, with Bravo (sent last) scoring
// highest, to prove Narrow both sorts by relevance_score and maps each
// result's index back to the candidate that document actually came from.
func TestNarrowSortsAnUnsortedRerankResponseAndMapsIndexesBackToEndpoints(t *testing.T) {
	alpha := endpoint("Alpha", "")
	charlie := endpoint("Charlie", "")
	bravo := endpoint("Bravo", "")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{alpha, charlie, bravo}}

	textAlpha := combinedText(&alpha)
	textCharlie := combinedText(&charlie)
	textBravo := combinedText(&bravo)

	embedResponses := map[string][]float32{
		"query: 質問":               {1, 0},
		"passage: " + textAlpha:   {1, 0},
		"passage: " + textCharlie: {0.8, 0.6},
		"passage: " + textBravo:   {0, 1},
	}

	// documents arrive at the reranker as [Alpha, Charlie, Bravo] (embed
	// order above), but the fake answers unsorted and with Bravo (index 2)
	// scoring highest.
	unsorted := func(documents []string) []rerankWireItem {
		if !assert.Len(t, documents, 3) {
			return nil
		}

		return []rerankWireItem{
			{Index: 2, RelevanceScore: 0.99}, // Bravo
			{Index: 0, RelevanceScore: 0.50}, // Alpha
			{Index: 1, RelevanceScore: 0.75}, // Charlie
		}
	}

	fake := narrowServer(t, embedResponses, unsorted)
	defer fake.server.Close()

	n := llamaswap.New(fake.server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), catalog))

	got, err := n.Narrow(context.Background(), catalog, "質問", 3)
	require.NoError(t, err)

	require.Len(t, got.Endpoints, 3)
	assert.Equal(t, []string{"Bravo", "Charlie", "Alpha"}, operationIDs(got),
		"results must be sorted by relevance_score descending and mapped back to the right endpoint, regardless of wire order")

	// k trims the sorted result, not the wire order: only the two
	// best-scored endpoints come back.
	trimmed, err := n.Narrow(context.Background(), catalog, "質問", 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"Bravo", "Charlie"}, operationIDs(trimmed))
}

func operationIDs(c domain.Catalog) []string {
	ids := make([]string, len(c.Endpoints))
	for i := range c.Endpoints {
		ids[i] = c.Endpoints[i].OperationID
	}

	return ids
}

// TestNarrowEndpointWithNoExamplesStillWorks is Task 1 Step 3(c): an
// endpoint whose contract declares no x-orchestra-examples is embedded,
// scored and reranked on its own text alone, with nothing that assumes at
// least one example exists.
func TestNarrowEndpointWithNoExamplesStillWorks(t *testing.T) {
	solo := endpoint("Solo", "")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{solo}}
	text := combinedText(&solo)

	embedResponses := map[string][]float32{
		"query: 質問":        {1, 0},
		"passage: " + text: {1, 0},
	}

	fake := narrowServer(t, embedResponses, identityRerank)
	defer fake.server.Close()

	n := llamaswap.New(fake.server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), catalog))

	got, err := n.Narrow(context.Background(), catalog, "質問", 1)

	require.NoError(t, err)
	require.Len(t, got.Endpoints, 1)
	assert.Equal(t, "Solo", got.Endpoints[0].OperationID)
	assert.Equal(t, []string{text}, *fake.rerankDocuments)
}

// TestNarrowOnlyConsidersEndpointsStillInTheGivenCatalogue proves the
// permission filter Narrow's own doc comment describes: an endpoint Load
// embedded but that is absent from the catalogue a particular call is
// given (the caller's permission-narrowed one, docs/specs/auth.md) is
// never offered, scored or sent to the reranker at all.
func TestNarrowOnlyConsidersEndpointsStillInTheGivenCatalogue(t *testing.T) {
	visible := endpoint("Visible", "")
	hidden := endpoint("Hidden", "")
	loaded := domain.Catalog{Endpoints: []domain.Endpoint{visible, hidden}}
	textVisible := combinedText(&visible)
	textHidden := combinedText(&hidden)

	embedResponses := map[string][]float32{
		"query: 質問":               {1, 0},
		"passage: " + textVisible: {1, 0},
		"passage: " + textHidden:  {1, 0},
	}

	fake := narrowServer(t, embedResponses, identityRerank)
	defer fake.server.Close()

	n := llamaswap.New(fake.server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), loaded))

	permitted := domain.Catalog{Endpoints: []domain.Endpoint{visible}}
	got, err := n.Narrow(context.Background(), permitted, "質問", 5)

	require.NoError(t, err)
	assert.Equal(t, []string{"Visible"}, operationIDs(got))
	assert.Equal(t, []string{textVisible}, *fake.rerankDocuments)
}

// TestNarrowWithNoEndpointsPermittedCallsNothing is the degenerate case of
// the same filter: a catalogue narrowed to nothing has nothing to embed a
// question against, and Narrow must not call llama-swap at all.
func TestNarrowWithNoEndpointsPermittedCallsNothing(t *testing.T) {
	solo := endpoint("Solo", "")
	text := combinedText(&solo)

	var loaded bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if loaded {
			assert.Fail(t, "Narrow must not call llama-swap when nothing is permitted")

			return
		}
		loaded = true

		var req embedWireRequest
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			return
		}
		assert.Equal(t, []string{"passage: " + text}, req.Input, "this is Load's own call")

		writeJSON(t, w, map[string]any{"data": []embedWireItem{{Index: 0, Embedding: []float32{1, 0}}}})
	}))
	defer server.Close()

	n := llamaswap.New(server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), domain.Catalog{Endpoints: []domain.Endpoint{solo}}))

	got, err := n.Narrow(context.Background(), domain.Catalog{}, "質問", 5)

	require.NoError(t, err)
	assert.Empty(t, got.Endpoints)
}

// TestLoadWrapsAnEmbeddingsHTTPError is Task 1 Step 3(d): a non-2xx from
// /v1/embeddings reaches Load's caller, wrapped in ErrRequestFailed, not
// swallowed into an empty index.
func TestLoadWrapsAnEmbeddingsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"embedding model not loaded"}`)
	}))
	defer server.Close()

	n := llamaswap.New(server.URL, "embed-model", "rerank-model")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{endpoint("Solo", "")}}

	err := n.Load(context.Background(), catalog)

	require.Error(t, err)
	assert.ErrorIs(t, err, llamaswap.ErrRequestFailed)
}

// TestNarrowWrapsAnEmbeddingsHTTPError is the other half of Step 3(d): a
// question embedding failure at Narrow time is wrapped and returned too.
func TestNarrowWrapsAnEmbeddingsHTTPError(t *testing.T) {
	solo := endpoint("Solo", "")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{solo}}

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// Load's own call: succeed.
			var req embedWireRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
				return
			}
			data := []embedWireItem{{Index: 0, Embedding: []float32{1, 0}}}
			writeJSON(t, w, map[string]any{"data": data})

			return
		}

		// Narrow's own question embedding: fail.
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"embedding model swapped out"}`)
	}))
	defer server.Close()

	n := llamaswap.New(server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), catalog))

	_, err := n.Narrow(context.Background(), catalog, "質問", 1)

	require.Error(t, err)
	assert.ErrorIs(t, err, llamaswap.ErrRequestFailed)
}

// TestNarrowWrapsARerankHTTPError is Step 3(d) at the reranker: both
// embeddings calls succeed, but /v1/rerank answers with a non-2xx, which
// must still reach Narrow's caller wrapped, not swallowed.
func TestNarrowWrapsARerankHTTPError(t *testing.T) {
	solo := endpoint("Solo", "")
	catalog := domain.Catalog{Endpoints: []domain.Endpoint{solo}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/embeddings":
			var req embedWireRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
				return
			}
			data := make([]embedWireItem, len(req.Input))
			for i := range req.Input {
				data[i] = embedWireItem{Index: i, Embedding: []float32{1, 0}}
			}
			writeJSON(t, w, map[string]any{"data": data})
		case "/v1/rerank":
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, `{"error":"reranker unreachable"}`)
		default:
			assert.Failf(t, "unexpected request path", "%q", r.URL.Path)
		}
	}))
	defer server.Close()

	n := llamaswap.New(server.URL, "embed-model", "rerank-model")
	require.NoError(t, n.Load(context.Background(), catalog))

	_, err := n.Narrow(context.Background(), catalog, "質問", 1)

	require.Error(t, err)
	assert.ErrorIs(t, err, llamaswap.ErrRequestFailed)
}
