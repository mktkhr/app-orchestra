package llamaswap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
)

// maxErrorDetailBytes bounds how much of a failing response body is
// folded into an error, mirroring internal/adapter/planner/chat's own
// tradeoff.
const maxErrorDetailBytes = 4096

// ErrRequestFailed is wrapped into the error returned when llama-swap
// answers a /v1/embeddings or /v1/rerank call with a non-2xx status.
var ErrRequestFailed = errors.New("narrowing request failed")

// ErrEmbeddingCountMismatch is returned when /v1/embeddings answers with a
// different number of vectors than texts were sent - a shape no
// OpenAI-compatible endpoint is documented to produce, but not one the
// wire format rules out either.
var ErrEmbeddingCountMismatch = errors.New("embeddings response returned a different number of vectors than texts sent")

// embedRequest is the JSON body of one /v1/embeddings call: an
// OpenAI-compatible request naming the model and every input at once, so
// Load posts the whole catalogue's documents in a single call (Task 1 Step
// 3(a): "posts every operation's text and each of its examples as
// documents once").
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedResponseItem is one entry of an embeddings response's data array.
// Index is read explicitly, not assumed to match position: llama-swap
// makes no promise data arrives in request order (mirroring
// e2e/narrowing/embedding/client.ts's own parseEmbeddingsResponse).
type embedResponseItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// embedResponse is the JSON body of a /v1/embeddings response.
type embedResponse struct {
	Data []embedResponseItem `json:"data"`
}

// normalise scales vector to unit length so a later dot product is a
// cosine similarity - the zero vector is returned unchanged, mirroring
// e2e/narrowing/embedding/client.ts's own normalise.
func normalise(vector []float32) []float32 {
	var sumSquares float64

	for _, v := range vector {
		sumSquares += float64(v) * float64(v)
	}

	if sumSquares == 0 {
		return vector
	}

	magnitude := math.Sqrt(sumSquares)
	out := make([]float32, len(vector))

	for i, v := range vector {
		out[i] = float32(float64(v) / magnitude)
	}

	return out
}

// embed posts texts to baseURL's /v1/embeddings with model, each prefixed
// by prefix (documentPrefix or queryPrefix), and returns one normalised
// vector per text, in the same order texts was given - regardless of the
// order the response's data array actually arrived in.
//
// Returns (nil, nil) for no texts, without calling the transport at all -
// the same shortcut e2e/narrowing/rerank/client.ts's own rerank takes for
// no candidates.
func embed(ctx context.Context, client *http.Client, baseURL, model, prefix string, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	prefixed := make([]string, len(texts))
	for i, t := range texts {
		prefixed[i] = prefix + t
	}

	body, err := json.Marshal(embedRequest{Model: model, Input: prefixed})
	if err != nil {
		return nil, fmt.Errorf("encoding embeddings request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/v1/embeddings", bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("building embeddings request: %w", err)
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

	var wire embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, fmt.Errorf("decoding embeddings response: %w", err)
	}

	if len(wire.Data) != len(texts) {
		return nil, fmt.Errorf("%w: got %d for %d", ErrEmbeddingCountMismatch, len(wire.Data), len(texts))
	}

	sort.Slice(wire.Data, func(i, j int) bool { return wire.Data[i].Index < wire.Data[j].Index })

	vectors := make([][]float32, len(wire.Data))
	for i, item := range wire.Data {
		vectors[i] = normalise(item.Embedding)
	}

	return vectors, nil
}

// requestError builds the error for a non-2xx response, folding in as
// much of the body as maxErrorDetailBytes allows - mirrors
// internal/adapter/planner/chat's own requestError.
func requestError(req *http.Request, resp *http.Response) error {
	detail, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorDetailBytes))
	if readErr != nil {
		detail = nil
	}

	return fmt.Errorf("%w: %s -> %d: %s", ErrRequestFailed, req.URL, resp.StatusCode, detail)
}
