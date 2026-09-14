import { expect, test } from "vite-plus/test";

import type { CatalogueVectors } from "../embedding/cache.ts";
import type { UtteranceVectors } from "../embedding/utterance-cache.ts";
import { narrow } from "./narrower.ts";
import type { NarrowResult } from "../lexical.ts";

/**
 * The utterance narrower's own tests (docs/plans/describing.md Task 2 Step
 * 1): a fake, tiny catalogue where the max-not-mean rule can be worked out
 * by hand, rather than the real fixture and a real model.
 */

// opA's own text points away from the question; one of its utterances
// points exactly at it. opB's own text is closer to the question than
// opA's own text, but has no utterance at all.
const VECTORS: CatalogueVectors = new Map([
  ["opA", [-1, 0, 0]],
  ["opB", [0.5, 0.5, 0]],
]);

const UTTERANCES: UtteranceVectors = new Map([["opA", [[1, 0, 0]]]]);

const QUESTION: readonly number[] = [1, 0, 0];

/** `ranked`'s score for `operationId`, or `Number.NaN` when it is not in `ranked` at all — a plain helper, not a test body, so it may branch. */
function scoreOf(ranked: readonly NarrowResult[], operationId: string): number {
  const found = ranked.find((r) => r.operationId === operationId);

  return found === undefined ? Number.NaN : found.score;
}

test("an operation whose utterance matches the question outscores one whose own text is closer to a different question", () => {
  const ranked = narrow(VECTORS, UTTERANCES, QUESTION, 2);

  expect(ranked.map((r) => r.operationId)).toEqual(["opA", "opB"]);
});

test("the score is the max over an operation's own vector and its utterances, not their sum or mean", () => {
  const ranked = narrow(VECTORS, UTTERANCES, QUESTION, 2);

  // dot([1,0,0],[1,0,0]) = 1, the utterance's own score - not a sum with
  // opA's own vector's score (dot([-1,0,0],[1,0,0]) = -1, which would pull
  // a sum or mean below opB's 0.5) and not a mean of the two (which would
  // land at 0).
  expect(scoreOf(ranked, "opA")).toBe(1);
});

test("an operation with no utterances is scored on its own vector alone", () => {
  const ranked = narrow(VECTORS, UTTERANCES, QUESTION, 2);

  expect(scoreOf(ranked, "opB")).toBe(0.5);
});

test("adding more utterances to an operation never lowers its score", () => {
  const oneUtterance: UtteranceVectors = new Map([["opB", [[0, 1, 0]]]]);
  const twoUtterances: UtteranceVectors = new Map([
    [
      "opB",
      [
        [0, 1, 0],
        [0, 0, 1],
      ],
    ],
  ]);

  const before = scoreOf(narrow(VECTORS, oneUtterance, QUESTION, 2), "opB");
  const after = scoreOf(narrow(VECTORS, twoUtterances, QUESTION, 2), "opB");

  expect(after >= before).toBe(true);
});

test("a third, weaker utterance never lowers an already-high score", () => {
  const strong: UtteranceVectors = new Map([["opA", [[1, 0, 0]]]]);
  const strongPlusWeak: UtteranceVectors = new Map([
    [
      "opA",
      [
        [1, 0, 0],
        [-0.9, 0, 0],
      ],
    ],
  ]);

  const before = scoreOf(narrow(VECTORS, strong, QUESTION, 2), "opA");
  const after = scoreOf(narrow(VECTORS, strongPlusWeak, QUESTION, 2), "opA");

  expect(after).toBe(before);
});
