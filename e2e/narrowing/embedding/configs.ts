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
    id: "qwen3-embedding-0.6b-q8",
    model: "qwen3-embedding-0.6b-q8",
    endpoint: "/v1/embeddings",
    pooling: "last",
    queryPrefix: "Instruct: …\nQuery: ",
    documentPrefix: "",
  },
] as const;
