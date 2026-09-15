/**
 * The `+reranker(w)+written` row's phase step (TODO.md item 1), pulled out
 * of `gather-phases.ts` only for eslint's `max-lines` - that file was
 * already near its budget before this row existed. Called from
 * `gather-phases.ts`'s `runRerankedRows`, exactly where the other reranked
 * rows run, with the same "still reachable" threading every phase uses.
 */
import type { K } from "./recall.ts";
import type { Question } from "./corpus/index.ts";
import type { GenerationPhaseResult } from "./gather-phases.ts";
import {
  unreachableReason,
  type ConfigurationResult,
  type GatherOptions,
  type SkippedConfiguration,
} from "./gather-helpers.ts";
import {
  TWO_STAGE_WRITTEN_RERANKER_CONFIG_ID,
  measureTwoStageWithWrittenRerankerConfiguration,
} from "./utterances/index.ts";

/**
 * `+reranker(w)+written`: the written-layer retrieval reranked on a
 * document that also carries the written examples
 * (`utterances/reranker-written.ts`), placed beside `+written` for direct
 * comparison and carrying its own contract rates the same way `+written`
 * does.
 */
export async function runWrittenRerankerRow(
  phase: GenerationPhaseResult,
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
  configurations: ConfigurationResult[],
  skipped: SkippedConfiguration[],
  reachable: boolean,
): Promise<boolean> {
  if (!reachable) {
    skipped.push({
      configId: TWO_STAGE_WRITTEN_RERANKER_CONFIG_ID,
      reason: "llama-swap was unreachable for an earlier configuration; not retried",
    });

    return false;
  }

  try {
    configurations.push(
      await measureTwoStageWithWrittenRerankerConfiguration(
        TWO_STAGE_WRITTEN_RERANKER_CONFIG_ID,
        phase.writtenUtterances,
        testQuestions,
        kValues,
        options,
      ),
    );

    return true;
  } catch (error) {
    skipped.push({
      configId: TWO_STAGE_WRITTEN_RERANKER_CONFIG_ID,
      reason: unreachableReason(error),
    });

    return false;
  }
}
