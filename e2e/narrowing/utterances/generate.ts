/**
 * The generated layer (docs/specs/describing.md section 3, section 5): one
 * llama-swap chat call per operation, asking the local model for five short
 * everyday Japanese phrasings of the operation's own text.
 *
 * The prompt is data, not code - it is part of the cache key (`cache.ts`) -
 * and it exists to close the four gaps that motivated this subproject
 * (spec section 1, DECISIONS.md 2026-09-14/2026-09-15): none of these four
 * questions share vocabulary with the operation they answer, so no amount
 * of lexical or embedding search over the operation's own words reaches it.
 *   - 「立て替えた分を出したい」 -> 経費申請の作成, ranked 385th by e5-large-q8
 *   - 「お金を返してもらいたい」 -> 精算の作成, 367th
 *   - 「商品が届いたので受け取り処理をしたい」 -> 検収の作成, 257th
 *   - 「値段を安くしてほしいと頼みたい」 -> 値引の作成, 111th
 *
 * The first version of this prompt ("この操作を...短い言い方を五つ") did not
 * close that gap - it widened it into the cache. Read back after generating
 * all 1000: createExpenseClaim -> 「経費申請作って / 経費申請作成 /
 * 経費申請作りたい / 経費申請作ります / 経費申請作れ」,
 * createPurchasingGoodsReceipt -> 「検収作成して / 検収作って / 検収作れ /
 * 検収作成 / 検収作」. Across all 4,960 utterances that run produced, none of
 * 立て替え, 受け取, 安く, 届いた or 休みたい appeared even once: the model had
 * conjugated each operation's own noun rather than describing the situation
 * that leads to it - a paraphrase of the summary in the summary's own
 * words, exactly what the retriever already reads, so it could not bridge
 * anything (the trap `contract.ts`'s novelty check now catches).
 *
 * This prompt instead few-shots the model with two examples from domains
 * outside the fixture (meeting-room booking, badge reissue) and asks for
 * the words of the person's own trouble, not the operation's. The two
 * example domains are deliberately foreign to the five fixture services -
 * fixture-domain examples would just teach the model to paraphrase harder
 * within the same vocabulary it already has.
 */
import { DEFAULT_BASE_URL, type FetchLike } from "../embedding/client.ts";

/** The picker's model (docs/plans/describing.md Task 1 Step 3), thinking off. */
export const UTTERANCE_MODEL = "qwen3.5-9b-q8";

/** One Japanese constant: `操作:` and the operation's own text follow it, on their own lines. */
export const UTTERANCE_PROMPT = `社内システムの操作ごとに、その操作を必要とする人が最初に言いそうな一言を集めています。システムの用語ではなく、その人の困りごとの言葉で。

例:
操作「会議室予約の作成」→「打ち合わせの部屋を押さえたい」「来週の会議どこでやろう」「10人入る部屋空いてる？」
操作「社員証の再発行」→「カードなくした」「入館証が読まなくなった」「ゲート通れない」

次の操作について、同じように五つ、一行に一つずつ、操作名の言葉を使わずに書いてください。他には何も書かない。`;

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
      messages: [{ role: "user", content: `${prompt}\n\n操作:\n${text}` }],
    }),
  });
  const body: unknown = await response.json();
  const { content } = parseChatResponse(body);

  return linesOf(content);
}
