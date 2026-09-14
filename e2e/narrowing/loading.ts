/**
 * Measures the alternation cost llama-swap imposes in its default mode -
 * one model resident at a time, no `groups` configured (docs/specs/
 * retrieving.md section 5). This is the one place in the whole subproject
 * that calls a chat model, and it never reads what the model said: only
 * the wall-clock matters, because what is being measured is the loading,
 * not the answer (docs/plans/retrieving.md Task 4 Step 1). It is
 * deliberately not exported from `index.ts` anywhere, has no test file
 * (there is nothing about it a fake transport could usefully check - the
 * cost it measures only exists against the real llama-swap), and is called
 * exactly once, from `measure.ts`'s `main`, so it never runs under `make
 * check` (AC-V-106).
 *
 * The five calls run in this order on purpose: warm the embedder
 * (untimed), time it resident, then a chat call (unloads the embedder),
 * then a chat call resident, then an embedding call (unloads the chat
 * model), then an embedding call resident again - reproducing the
 * alternation a live deployment would see if narrowing and answering
 * shared one llama-swap instance (spec section 5).
 */
import { EMBEDDING_CONFIGS } from "./embedding/index.ts";
import { embedQuestion, DEFAULT_BASE_URL, type FetchLike } from "./embedding/client.ts";

/** The chat model behind `/v1/chat/completions` in llama-swap's current configuration (docs/plans/retrieving.md Task 4). */
export const CHAT_MODEL = "qwen3.5-9b-q8";

/** The embedding configuration used to probe loading - any would do; `e5-large-q8` is already resident from Task 2/3's own measurement. */
const PROBE_EMBEDDING_CONFIG_ID = "e5-large-q8";

/** Content is never read - only the wall-clock of getting an answer at all is measured (this file's header). */
const PROBE_CHAT_MESSAGE = "こんにちは";

/** The five measurements from docs/plans/retrieving.md Task 4 Step 1, in seconds. */
export interface AlternationResult {
  readonly embeddingResident: number;
  readonly chatUnloadingEmbedder: number;
  readonly chatResident: number;
  readonly embeddingUnloadingChat: number;
  readonly embeddingResidentAgain: number;
}

function probeEmbeddingConfig(): (typeof EMBEDDING_CONFIGS)[number] {
  const found = EMBEDDING_CONFIGS.find((config) => config.id === PROBE_EMBEDDING_CONFIG_ID);

  if (found === undefined) {
    throw new Error(`loading.ts: no embedding configuration named ${PROBE_EMBEDDING_CONFIG_ID}`);
  }

  return found;
}

/** Seconds elapsed running `call`, discarding whatever it resolves to. */
async function timedSeconds(call: () => Promise<unknown>): Promise<number> {
  const start = Date.now();

  await call();

  return (Date.now() - start) / 1000;
}

/**
 * One `/v1/chat/completions` call with `max_tokens: 1` - only its
 * wall-clock is used by any caller of this function; the response body is
 * read (so the connection is drained) but never inspected.
 */
async function chatOnce(fetchImpl: FetchLike, baseUrl: string): Promise<void> {
  const response = await fetchImpl(`${baseUrl}/v1/chat/completions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      model: CHAT_MODEL,
      messages: [{ role: "user", content: PROBE_CHAT_MESSAGE }],
      max_tokens: 1,
    }),
  });

  await response.json();
}

/**
 * Runs the five-call sequence documented at this file's top and returns
 * each leg's wall-clock in seconds. Talks to the real llama-swap by
 * default (`DEFAULT_BASE_URL`); `fetchImpl`/`baseUrl` exist only so this
 * function itself stays testable in principle, not because anything in
 * this subproject's test suite calls it.
 */
export async function measureAlternation(
  fetchImpl: FetchLike = fetch,
  baseUrl: string = DEFAULT_BASE_URL,
): Promise<AlternationResult> {
  const config = probeEmbeddingConfig();

  // 1. Warm the embedder (untimed), then time a second call with it
  // already resident.
  await embedQuestion(config, PROBE_CHAT_MESSAGE, fetchImpl, baseUrl);
  const embeddingResident = await timedSeconds(() =>
    embedQuestion(config, PROBE_CHAT_MESSAGE, fetchImpl, baseUrl),
  );

  // 2. A chat call - unloads the embedder first.
  const chatUnloadingEmbedder = await timedSeconds(() => chatOnce(fetchImpl, baseUrl));

  // 3. A chat call with the chat model now resident.
  const chatResident = await timedSeconds(() => chatOnce(fetchImpl, baseUrl));

  // 4. An embedding call - unloads the chat model first.
  const embeddingUnloadingChat = await timedSeconds(() =>
    embedQuestion(config, PROBE_CHAT_MESSAGE, fetchImpl, baseUrl),
  );

  // 5. An embedding call with the embedder resident again.
  const embeddingResidentAgain = await timedSeconds(() =>
    embedQuestion(config, PROBE_CHAT_MESSAGE, fetchImpl, baseUrl),
  );

  return {
    embeddingResident,
    chatUnloadingEmbedder,
    chatResident,
    embeddingUnloadingChat,
    embeddingResidentAgain,
  };
}
