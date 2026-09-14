/**
 * The generated layer (docs/specs/describing.md section 3, section 5): one
 * llama-swap chat call per operation, asking the local model for five short
 * everyday Japanese phrasings of the operation's own text.
 *
 * The prompt is data, not code - it is part of the cache key (`cache.ts`),
 * and it exists to close the four gaps that motivated this subproject
 * (spec section 1, DECISIONS.md 2026-09-14/2026-09-15): none of these four
 * questions share vocabulary with the operation they answer, so no amount
 * of lexical or embedding search over the operation's own words reaches it.
 *   - 「立て替えた分を出したい」 -> 経費申請の作成, ranked 385th by e5-large-q8
 *   - 「お金を返してもらいたい」 -> 精算の作成, 367th
 *   - 「商品が届いたので受け取り処理をしたい」 -> 検収の作成, 257th
 *   - 「値段を安くしてほしいと頼みたい」 -> 値引の作成, 111th
 * A generator that only rephrases the operation's own words would not close
 * this kind of gap; the prompt asks for everyday language on purpose, and
 * gives the model nothing the retriever does not also see (G3, G4).
 *
 * Thinking eats `max_tokens` and has already burned two runs (local,
 * 2026-09-14; API, 2026-09-15) - `chat_template_kwargs.enable_thinking` is
 * set to `false` and `temperature` to `0` explicitly on every request,
 * never left to the model's default. Transport is injectable exactly as
 * `embedding/client.ts` does it, so `make check` calls no model of any kind.
 */
import { DEFAULT_BASE_URL, type FetchLike } from "../embedding/client.ts";

/** The picker's model (docs/plans/describing.md Task 1 Step 3), thinking off. */
export const UTTERANCE_MODEL = "qwen3.5-9b-q8";

/** One Japanese constant: the operation's own text is appended after it. */
export const UTTERANCE_PROMPT =
  "この操作を、社内の人が日常語で頼むとしたら、どう言うか。短い言い方を五つ、一行に一つずつ、他には何も書かずに挙げてください。";

/** Enough for five short lines, not enough to hide a thinking block inside. */
export const UTTERANCE_MAX_TOKENS = 300;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

interface ChatCompletionResult {
  readonly content: string;
  readonly finishReason: string | undefined;
}

/** `body` read as an OpenAI-compatible chat completion response, by runtime check. */
function parseChatResponse(body: unknown): ChatCompletionResult {
  if (!isRecord(body)) throw new Error("chat completion response was not a JSON object");

  const { choices } = body;

  if (!Array.isArray(choices) || choices.length === 0) {
    throw new Error("chat completion response had no choices");
  }

  const first: unknown = choices[0];

  if (!isRecord(first))
    throw new Error("chat completion response's first choice was not a JSON object");

  const { message, finish_reason: finishReasonRaw } = first;

  if (!isRecord(message)) throw new Error("chat completion response had no message");

  const { content } = message;

  if (typeof content !== "string")
    throw new Error("chat completion response had no string content");

  return {
    content,
    finishReason: typeof finishReasonRaw === "string" ? finishReasonRaw : undefined,
  };
}

/** `content` split into trimmed, non-blank lines - one per phrasing, in the order the model wrote them. */
function linesOf(content: string): readonly string[] {
  return content
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

/**
 * `text` (an operation's `combinedTextOf`) turned into short everyday
 * phrasings by one chat call. Fewer than five lines are returned as-is -
 * not padded, not retried, because a generator that pads or retries is
 * measuring its own retry policy rather than what the model actually wrote.
 */
export async function generateUtterancesFor(
  text: string,
  prompt: string = UTTERANCE_PROMPT,
  fetchImpl: FetchLike = fetch,
  baseUrl: string = DEFAULT_BASE_URL,
): Promise<readonly string[]> {
  const response = await fetchImpl(`${baseUrl}/v1/chat/completions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      model: UTTERANCE_MODEL,
      temperature: 0,
      max_tokens: UTTERANCE_MAX_TOKENS,
      chat_template_kwargs: { enable_thinking: false },
      messages: [{ role: "user", content: `${prompt}\n\n${text}` }],
    }),
  });
  const body: unknown = await response.json();
  const { content } = parseChatResponse(body);

  return linesOf(content);
}
