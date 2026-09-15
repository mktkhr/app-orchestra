package llamaswap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ErrRerankCountMismatch is returned when /v1/rerank answers with a
// different number of results than documents were sent.
var ErrRerankCountMismatch = errors.New("rerank response returned a different number of results than documents sent")

// ErrRerankIndexOutOfRange is returned when a /v1/rerank result names an
// index outside the documents that were actually sent - a malformed
// response this package refuses to guess its way past.
var ErrRerankIndexOutOfRange = errors.New("rerank response referenced a document index out of range")

// rerankRequest is the JSON body of one /v1/rerank call.
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

// rerankResultItem is one entry of a /v1/rerank response's results array:
// Index is the position of a document in the Documents array that was
// sent (docs/plans/shortlisting.md, Task 1 Step 3(b)); RelevanceScore is
// what results are ranked by.
type rerankResultItem struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// rerankResponse is the JSON body of a /v1/rerank response. Its Results
// arrive in no promised order - bge-reranker-v2-m3-q8 over llama-swap
// answers unsorted (e2e/narrowing/rerank/client.ts's own doc comment,
// "the bug this task exists to avoid") - rerankCandidates below sorts them
// itself, by RelevanceScore, only after mapping every result's Index back
// to the id it named.
type rerankResponse struct {
	Results []rerankResultItem `json:"results"`
}

// rerankedID is one document's outcome: the id its caller gave it (an
// endpointID, opaque to this function) and its relevance score.
type rerankedID struct {
	id    endpointID
	score float64
}

// rerankCandidates reranks documents (each already the full text a
// candidate should be scored on - own text plus its examples, joined) for
// query, and returns one rerankedID per document, sorted best (highest
// relevance_score) first. ids[i] must name the candidate documents[i]
// came from.
//
// Returns nil for no documents, without calling the transport at all -
// the same shortcut e2e/narrowing/rerank/client.ts's own rerank takes.
func rerankCandidates(
	ctx context.Context, client *http.Client, baseURL, model, query string, ids []endpointID, documents []string,
) ([]rerankedID, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(rerankRequest{Model: model, Query: query, Documents: documents})
	if err != nil {
		return nil, fmt.Errorf("encoding rerank request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/v1/rerank", bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("building rerank request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, requestError(req, resp)
	}

	var wire rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, fmt.Errorf("decoding rerank response: %w", err)
	}

	if len(wire.Results) != len(documents) {
		return nil, fmt.Errorf("%w: got %d for %d", ErrRerankCountMismatch, len(wire.Results), len(documents))
	}

	results := make([]rerankedID, len(wire.Results))

	for i, item := range wire.Results {
		if item.Index < 0 || item.Index >= len(ids) {
			return nil, fmt.Errorf("%w: %d", ErrRerankIndexOutOfRange, item.Index)
		}

		results[i] = rerankedID{id: ids[item.Index], score: item.RelevanceScore}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	return results, nil
}
