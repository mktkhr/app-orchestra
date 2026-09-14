/**
 * The embedding configurations this subproject measures
 * (docs/specs/retrieving.md section 3, docs/plans/retrieving.md's llama-swap
 * table). Pure data - no logic - so `client.ts`, `cache.ts` and `contract.ts`
 * all read the same table rather than each hard-coding their own idea of it.
 *
 * `ruri-v3-310m-q8` (CLS pooling) is kept beside `ruri-v3-310m-q8-mean`
 * (mean pooling) on purpose: it is the evidence for spec decision V2 - the
 * same model, wrongly pooled, scores 6/25 on axis B where the correctly
 * pooled one scores 22/25 - and removing it would leave that decision
 * looking arbitrary.
 */

/** How a model's output vectors are reduced to one vector per input. */
export type Pooling = "cls" | "mean" | "last";

/** One embedding model's contract with llama-swap. */
export interface EmbeddingConfig {
  readonly id: string;
  readonly model: string;
  readonly endpoint: "/v1/embeddings";
  readonly pooling: Pooling;
  readonly queryPrefix: string;
  readonly documentPrefix: string;
}

/**
 * Six configurations (spec section 3's table minus the retrieve-then-rerank
 * row, which Task 3 adds as a two-stage narrower rather than a fetchable
 * embedding config). `qwen3-embedding-0.6b-q8` and `bge-m3-q8` are the two
 * spec section 4 flags as unverified - `checkContract` is what verifies
 * them.
 */
export const EMBEDDING_CONFIGS: readonly EmbeddingConfig[] = [
  {
    id: "bge-m3-q8",
    model: "bge-m3-q8",
    endpoint: "/v1/embeddings",
    pooling: "cls",
    queryPrefix: "",
    documentPrefix: "",
  },
  {
    id: "e5-large-q8",
    model: "e5-large-q8",
    endpoint: "/v1/embeddings",
    pooling: "mean",
    queryPrefix: "query: ",
    documentPrefix: "passage: ",
  },
  {
    id: "ruri-v3-310m-q8-mean",
    model: "ruri-v3-310m-q8-mean",
    endpoint: "/v1/embeddings",
    pooling: "mean",
    queryPrefix: "検索クエリ: ",
    documentPrefix: "検索文書: ",
  },
  {
    id: "ruri-v3-310m-q8",
    model: "ruri-v3-310m-q8",
    endpoint: "/v1/embeddings",
    pooling: "cls",
    queryPrefix: "検索クエリ: ",
    documentPrefix: "検索文書: ",
  },
  {
    // The instruction is the model's own contract, and "…" was a placeholder
    // in docs/plans/retrieving.md's table that should never have been copied
    // into data. Measured 2026-09-14: with the placeholder, and with this real
    // instruction, this configuration retrieves a document by its own text
    // less reliably than the same model with no query prefix at all - which is
    // why the next entry exists rather than this one being "fixed".
    id: "qwen3-embedding-0.6b-q8",
    model: "qwen3-embedding-0.6b-q8",
    endpoint: "/v1/embeddings",
    pooling: "last",
    queryPrefix: "Instruct: Given a question, retrieve the API operation that answers it\nQuery: ",
    documentPrefix: "",
  },
  {
    // The same model with no query prefix. An instruction on the query side
    // and nothing on the document side is an asymmetry, and whether it helps
    // or hurts is a question for the recall table rather than for the model
    // card (docs/specs/retrieving.md V2: configurations are what is measured).
    id: "qwen3-embedding-0.6b-q8-plain",
    model: "qwen3-embedding-0.6b-q8",
    endpoint: "/v1/embeddings",
    pooling: "last",
    queryPrefix: "",
    documentPrefix: "",
  },
] as const;
