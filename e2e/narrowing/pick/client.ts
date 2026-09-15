/**
 * Talks to llama-swap's `/v1/chat/completions` with `qwen3.5-9b-q8`,
 * thinking off (TODO.md "Measure the pick, not only the recall";
 * DECISIONS.md 2026-09-15, "The local picker reads the shortlist in
 * reranker order: 78 → 83, for nothing" - the hand measurement this client
 * turns into a proper report row). One call per question: the system
 * prompt below is copied verbatim from that hand-run script, and the
 * parsing rule is the same one it used - whichever candidate id appears in
 * the response text, longest id first so one id being a substring of
 * another cannot steal the match, and `ambiguous` if that word appears
 * anywhere in the response.
 *
 * Transport is injectable, the same shape `embedding/client.ts` and
 * `rerank/client.ts` use - a `fetch`-shaped function defaulted to the real
 * `fetch`, so `make check` never calls a model (AC-V-106).
 */
import { DEFAULT_BASE_URL, type FetchLike } from "../embedding/client.ts";

/** The chat model behind the picker (DECISIONS.md 2026-09-15). */
export const PICK_MODEL = "qwen3.5-9b-q8";

/** Copied verbatim from the hand-run script that produced the 83 in DECISIONS.md 2026-09-15. Left byte-for-byte unchanged (TODO.md item 1): the three pre-existing pick rows (`pick:e5-large-q8+reranker`, `+written`, `+both`) must keep using exactly this text, since a measurement taken under a different prompt is not comparable to the one already recorded for them. */
export const PICK_SYSTEM_PROMPT = `あなたは社内APIの振り分け役。質問に対して、候補一覧の中から呼ぶべきAPIを1つ選ぶ。

出力は次の形式の1行だけ。説明もタグも書かない。
listInventoryItems certain

1語目は候補一覧にある operationId をそのまま。2語目は certain か ambiguous。
ambiguous は「質問文だけでは候補を1つに決められない」場合。自信の有無ではなく、質問が足りていない場合。
ambiguous のときも、最も可能性の高い operationId を必ず1つ挙げること。`;

/**
 * `PICK_SYSTEM_PROMPT` plus one added sentence (TODO.md item 1): the
 * pick-side half of "let the reranker read the written examples" - a
 * candidate line can now carry an `e.g.` column of an operation's written
 * examples (`candidateLine` below), and without being told what that
 * column is, the picker has no reason to weigh it.
 *
 * Deliberately a *separate* constant from `PICK_SYSTEM_PROMPT` rather than
 * an edit to it: an earlier version of this change edited
 * `PICK_SYSTEM_PROMPT` in place, which silently changed the prompt every
 * pick row uses, including the three pre-existing ones that show no `e.g.`
 * column at all - re-running the full report moved
 * `pick:e5-large-q8+reranker` from 83%/51% correct/flagged to 77%/47% correct/flagged
 * on that one added sentence alone, overall and per axis, with the
 * underlying shortlist unchanged (its own recall row is byte-identical
 * before and after). `pick/index.ts`'s `pick` takes this prompt as an
 * explicit parameter, defaulting to `PICK_SYSTEM_PROMPT`, so a caller has
 * to opt in to the extended one rather than get it by accident - only
 * `pick:e5-large-q8+reranker(w)+written` and
 * `pick:e5-large-q8+reranker+written(shown)` do (`gather-pick.ts`).
 */
export const PICK_SYSTEM_PROMPT_WITH_EXAMPLES = `${PICK_SYSTEM_PROMPT}
候補に e.g. 列がある場合、それはそのAPIに対して人がよく尋ねる質問の例。`;

/** One candidate the picker is offered, in the same order the reranker returned it. `examples`, when present and non-empty, is shown as an extra `e.g.` column (`candidateLine` below) — TODO.md item 1. */
export interface PickCandidate {
  readonly operationId: string;
  readonly serviceDisplayName: string;
  readonly summary: string;
  readonly examples?: readonly string[];
}

/** The picker's outcome for one question. */
export interface PickResult {
  readonly operationId: string;
  readonly ambiguous: boolean;
}

/**
 * Whether the thinking-budget guard (this file's header) has already run
 * once this run - mutated by `pick` below, one instance per caller. Kept as
 * a plain mutable object rather than module-level state so a test can start
 * a fresh guard per case instead of depending on suite ordering.
 */
export interface PickGuardState {
  checked: boolean;
}

/** A fresh, unchecked guard state - one per `gatherReport` run (`gather-pick.ts`). */
export function newPickGuardState(): PickGuardState {
  return { checked: false };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

interface ParsedChatResponse {
  readonly content: string;
  readonly finishReason: string | undefined;
  readonly reasoningContent: string | undefined;
}

/** `body` read as an OpenAI-compatible chat-completion response's first choice. */
function parseChatResponse(body: unknown): ParsedChatResponse {
  if (!isRecord(body)) throw new Error("chat completion response was not a JSON object");

  const { choices } = body;

  if (!Array.isArray(choices)) throw new Error("chat completion response had no choices array");

  const first: unknown = choices[0];

  if (!isRecord(first)) throw new Error("chat completion response had no first choice");

  const { message, finish_reason: finishReasonRaw } = first;

  if (!isRecord(message)) throw new Error("chat completion response choice had no message");

  const { content: contentRaw, reasoning_content: reasoningRaw } = message;

  return {
    content: typeof contentRaw === "string" ? contentRaw : "",
    finishReason: typeof finishReasonRaw === "string" ? finishReasonRaw : undefined,
    reasoningContent: typeof reasoningRaw === "string" ? reasoningRaw : undefined,
  };
}

/**
 * The extra safety net beside `chat_template_kwargs.enable_thinking: false`
 * (this file's header): thinking eats into `max_tokens` (200) before the
 * model ever writes the operationId, so a leak would silently truncate the
 * answer rather than throw on its own. Checked once, on the first real
 * response of a run (`pick` below) - never on every question, and never
 * against a fake transport a test hands it.
 */
function assertNoThinkingLeak(parsed: ParsedChatResponse): void {
  const reasoningLeaked =
    parsed.reasoningContent !== undefined && parsed.reasoningContent.length > 0;

  if (parsed.finishReason === "stop" && !reasoningLeaked) return;

  throw new Error(
    "pick/client.ts: thinking leaked into the response and is eating max_tokens — check " +
      `chat_template_kwargs (finish_reason=${String(parsed.finishReason)}, ` +
      `reasoning_content present=${String(reasoningLeaked)})`,
  );
}

/**
 * One candidate line: `operationId\tservice display name\tsummary`, plus a
 * trailing `\te.g. 例文1 / 例文2` when `candidate.examples` is present and
 * non-empty (TODO.md item 1) - omitted entirely otherwise, so the three
 * pre-existing pick rows' candidate lines are unchanged.
 */
function candidateLine(candidate: PickCandidate): string {
  const base = `${candidate.operationId}\t${candidate.serviceDisplayName}\t${candidate.summary}`;
  const examplesColumn =
    candidate.examples === undefined || candidate.examples.length === 0
      ? ""
      : `\te.g. ${candidate.examples.join(" / ")}`;

  return base + examplesColumn;
}

function userMessageFor(question: string, candidates: readonly PickCandidate[]): string {
  return `質問: ${question}\n\n候補:\n${candidates.map((candidate) => candidateLine(candidate)).join("\n")}`;
}

/**
 * The answer out of `content`: whichever candidate id appears in it, longest
 * id first so `listInventoryItems` cannot be stolen by a shorter id that
 * happens to be one of its substrings, `""` if none appear at all -
 * `ambiguous` is `true` whenever that word appears anywhere in the text,
 * independent of which id was found (the hand-run script's own rule).
 */
function parsePick(content: string, candidateIds: readonly string[]): PickResult {
  const longestFirst = candidateIds.toSorted((a, b) => b.length - a.length);
  const found = longestFirst.find((id) => content.includes(id));

  return { operationId: found ?? "", ambiguous: /ambiguous/iu.test(content) };
}

/**
 * Picks one operationId out of `candidates` for `question`, temperature 0,
 * thinking off. `guardState` is shared across every call in a run so the
 * leak assertion above runs on the first real response only - a test's fake
 * transport is checked the same way its own `guardState` says it should be,
 * so a test that wants to see the guard fire uses a fresh `newPickGuardState()`.
 * `systemPrompt` defaults to `PICK_SYSTEM_PROMPT` - pass
 * `PICK_SYSTEM_PROMPT_WITH_EXAMPLES` only for a variant whose candidates
 * carry the `e.g.` column (TODO.md item 1's byte-identity note above).
 */
export async function pick(
  question: string,
  candidates: readonly PickCandidate[],
  guardState: PickGuardState,
  fetchImpl: FetchLike = fetch,
  baseUrl: string = DEFAULT_BASE_URL,
  systemPrompt: string = PICK_SYSTEM_PROMPT,
): Promise<PickResult> {
  const response = await fetchImpl(`${baseUrl}/v1/chat/completions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      model: PICK_MODEL,
      temperature: 0,
      max_tokens: 200,
      chat_template_kwargs: { enable_thinking: false },
      messages: [
        { role: "system", content: systemPrompt },
        { role: "user", content: userMessageFor(question, candidates) },
      ],
    }),
  });
  const body: unknown = await response.json();
  const parsed = parseChatResponse(body);

  if (!guardState.checked) {
    guardState.checked = true;
    assertNoThinkingLeak(parsed);
  }

  return parsePick(
    parsed.content,
    candidates.map((candidate) => candidate.operationId),
  );
}
