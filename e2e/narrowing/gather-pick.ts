/**
 * The five pick rows (TODO.md "Measure the pick, not only the recall";
 * DECISIONS.md 2026-09-15, "The local picker reads the shortlist in
 * reranker order: 78 → 83, for nothing"): `qwen3.5-9b-q8`, thinking off,
 * picks one operationId out of the top `PICK_SHORTLIST_K` reranked
 * candidates - scored on whether the pick was right and whether it was
 * flagged `ambiguous`, not on recall@K.
 *
 * Two passes, deliberately, and in that order: every shortlist for every
 * variant is gathered first (`gather-pick-shortlists.ts`), reusing the
 * catalogue and utterance vectors the recall rows already cached on disk,
 * and only once that is done does the picker run at all
 * (`gather-pick-scoring.ts`) - so a run alternates between the reranker and
 * the chat model exactly once, not once per question (the same discipline
 * `gather-phases.ts`'s header states for retrieval vs. reranking, and
 * `docs/plans/describing.md`'s "Details that decide" states for generation
 * vs. embedding).
 *
 * `measure.ts`'s `gatherReport` calls `runPickRows` last, after every recall
 * row: the plain, `+written` and `+both` two-stage shortlists it reranks
 * read the same `e5-large-q8` catalogue vectors those rows already embedded,
 * so nothing here re-embeds the catalogue.
 *
 * TODO.md item 1 adds two rows, both against the written shortlist so the
 * two effects can be told apart: `PICK_WRITTEN_RERANKER_CONFIG_ID` (the
 * reranker reads the written examples, `utterances/reranker-written.ts`'s
 * shortlist, and the picker is shown them too) and
 * `PICK_WRITTEN_SHOWN_CONFIG_ID` (the plain `+written` shortlist,
 * unchanged, with the examples shown to the picker only). The three
 * pre-existing rows above are otherwise untouched.
 */
import { embedCatalogue, embedUtteranceVectors } from "./embedding/index.ts";
import type { FetchLike } from "./embedding/client.ts";
import { catalogOf, type FixtureOperation } from "./fixture/index.ts";
import type { Question } from "./corpus/index.ts";
import type { GenerationPhaseResult } from "./gather-phases.ts";
import {
  embedQuestionsOnce,
  retrieverConfig,
  transportOptionsOf,
  unreachableReason,
  type GatherOptions,
} from "./gather-helpers.ts";
import { gatherAllShortlists, type PickVariant } from "./gather-pick-shortlists.ts";
import { scorePickRows } from "./gather-pick-scoring.ts";
import type { PickConfigurationResult, SkippedConfiguration } from "./report-types.ts";

/** The plain two-stage shortlist, unmodified — the row DECISIONS.md 2026-09-15's hand measurement is checked against. */
export const PICK_PLAIN_CONFIG_ID = "pick:e5-large-q8+reranker";
/** The same shortlist, built from the written utterance layer (`utterances/rows.ts`'s `TWO_STAGE_WRITTEN_CONFIG_ID` shortlist). */
export const PICK_WRITTEN_CONFIG_ID = "pick:e5-large-q8+reranker+written";
/** The same shortlist, built from both utterance layers (`utterances/rows.ts`'s `TWO_STAGE_BOTH_CONFIG_ID` shortlist). */
export const PICK_BOTH_CONFIG_ID = "pick:e5-large-q8+reranker+both";
/** The written-layer shortlist reranked on the written examples (`utterances/reranker-written.ts`), with those same examples shown to the picker — TODO.md item 1's combined variant. */
export const PICK_WRITTEN_RERANKER_CONFIG_ID = "pick:e5-large-q8+reranker(w)+written";
/** The plain `+written` shortlist, unchanged, with the written examples shown to the picker only — TODO.md item 1's picker-only variant, to separate the two effects. */
export const PICK_WRITTEN_SHOWN_CONFIG_ID = "pick:e5-large-q8+reranker+written(shown)";

const PICK_CONFIG_IDS = [
  PICK_PLAIN_CONFIG_ID,
  PICK_WRITTEN_CONFIG_ID,
  PICK_BOTH_CONFIG_ID,
  PICK_WRITTEN_RERANKER_CONFIG_ID,
  PICK_WRITTEN_SHOWN_CONFIG_ID,
] as const;

function skipAllPickRows(skipped: SkippedConfiguration[], reason: string): void {
  for (const configId of PICK_CONFIG_IDS) skipped.push({ configId, reason });
}

/** Every pick row's inputs: the plain shortlist and the four utterance-scored ones, all against the same cached catalogue vectors and question vectors. */
async function pickVariantsOf(
  phase: GenerationPhaseResult,
  fullCatalog: readonly FixtureOperation[],
  transportOptions: {
    readonly dir?: string;
    readonly fetchImpl?: FetchLike;
    readonly baseUrl?: string;
  },
): Promise<readonly PickVariant[]> {
  const config = retrieverConfig();
  const writtenUtteranceVectors = await embedUtteranceVectors(
    config,
    "written",
    fullCatalog,
    phase.writtenUtterances,
    transportOptions,
  );
  const bothUtteranceVectors = await embedUtteranceVectors(
    config,
    "both",
    fullCatalog,
    phase.bothUtterances,
    transportOptions,
  );

  return [
    { configId: PICK_PLAIN_CONFIG_ID, utterances: undefined },
    { configId: PICK_WRITTEN_CONFIG_ID, utterances: writtenUtteranceVectors },
    { configId: PICK_BOTH_CONFIG_ID, utterances: bothUtteranceVectors },
    {
      configId: PICK_WRITTEN_RERANKER_CONFIG_ID,
      utterances: writtenUtteranceVectors,
      useWrittenReranker: true,
      showExamples: true,
    },
    {
      configId: PICK_WRITTEN_SHOWN_CONFIG_ID,
      utterances: writtenUtteranceVectors,
      showExamples: true,
    },
  ];
}

/**
 * Gathers and scores every pick row, appending to `pickConfigurations` when
 * the transport is reachable and recording all five as skipped, with the
 * same reason, otherwise — exactly like the other phases in
 * `gather-phases.ts`.
 */
export async function runPickRows(
  phase: GenerationPhaseResult,
  testQuestions: readonly Question[],
  options: GatherOptions,
  pickConfigurations: PickConfigurationResult[],
  skipped: SkippedConfiguration[],
  reachable: boolean,
): Promise<void> {
  if (!reachable) {
    skipAllPickRows(
      skipped,
      "llama-swap was unreachable for an earlier configuration; not retried",
    );

    return;
  }

  try {
    const config = retrieverConfig();
    const fullCatalog = catalogOf(5);

    const transportOptions = transportOptionsOf(options);
    const fullVectors = await embedCatalogue(config, fullCatalog, transportOptions);
    const questionVectors = await embedQuestionsOnce(config, testQuestions, transportOptions);
    const variants = await pickVariantsOf(phase, fullCatalog, transportOptions);
    const shortlists = await gatherAllShortlists(
      variants,
      fullVectors,
      questionVectors,
      testQuestions,
      transportOptions.fetchImpl,
      transportOptions.baseUrl,
    );
    const results = await scorePickRows(
      shortlists,
      transportOptions.fetchImpl,
      transportOptions.baseUrl,
    );

    pickConfigurations.push(...results);
  } catch (error) {
    skipAllPickRows(skipped, unreachableReason(error));
  }
}
