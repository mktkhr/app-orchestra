import { expect, test } from "vite-plus/test";

import { score } from "./score.ts";
import type { TurnRecord } from "./types.ts";

/**
 * score.ts's own tests: no server, no model - every TurnRecord here is a
 * fake, mirroring e2e/shortlist/score.test.ts's style.
 */

function turn(dialogueId: string, turnIndex: number, pass: boolean, latencyMs: number): TurnRecord {
  return {
    dialogueId,
    turnIndex,
    question: `q${String(turnIndex)}`,
    requestTurns: [],
    outcome: { kind: "result" },
    rawResponse: {},
    expected: [{ kind: "result" }],
    pass,
    latencyMs,
  };
}

test("all turns counts every record", () => {
  const records = [turn("d1", 0, true, 100), turn("d1", 1, false, 200)];

  expect(score(records).allTurns).toEqual({ correct: 1, total: 2 });
});

test("turns 2+ excludes each dialogue's first turn", () => {
  const records = [
    turn("d1", 0, false, 100),
    turn("d1", 1, true, 100),
    turn("d1", 2, true, 100),
    turn("d2", 0, true, 100),
  ];

  // d1's turnIndex 0 is excluded (its own failure does not count against
  // turns2Plus), turnIndex 1 and 2 do; d2 has no turn past its first.
  expect(score(records).turns2Plus).toEqual({ correct: 2, total: 2 });
});

test("conditional turns2+ only counts a turn whose every earlier turn in its dialogue passed", () => {
  const records = [
    // d1: turn0 fails, so turn1 and turn2 are never counted conditionally,
    // even though turn2 itself passes.
    turn("d1", 0, false, 100),
    turn("d1", 1, true, 100),
    turn("d1", 2, true, 100),
    // d2: every earlier turn passes, so turn1 and turn2 both count.
    turn("d2", 0, true, 100),
    turn("d2", 1, true, 100),
    turn("d2", 2, false, 100),
  ];

  expect(score(records).conditionalTurns2Plus).toEqual({ correct: 1, total: 2 });
});

test("dialogues all correct counts a dialogue only when every turn in it passed", () => {
  const records = [
    turn("d1", 0, true, 100),
    turn("d1", 1, true, 100),
    turn("d2", 0, true, 100),
    turn("d2", 1, false, 100),
  ];

  expect(score(records).dialoguesAllCorrect).toEqual({ correct: 1, total: 2 });
});

test("latency mean and p50", () => {
  const records = [turn("d1", 0, true, 100), turn("d1", 1, true, 300), turn("d1", 2, true, 200)];
  const result = score(records);

  expect(result.latencyMeanMs).toBeCloseTo(200, 5);
  expect(result.latencyP50Ms).toBe(200);
});

test("an empty list scores every fraction as 0/0 and latency as 0", () => {
  const result = score([]);

  expect(result.allTurns).toEqual({ correct: 0, total: 0 });
  expect(result.turns2Plus).toEqual({ correct: 0, total: 0 });
  expect(result.conditionalTurns2Plus).toEqual({ correct: 0, total: 0 });
  expect(result.dialoguesAllCorrect).toEqual({ correct: 0, total: 0 });
  expect(result.latencyMeanMs).toBe(0);
  expect(result.latencyP50Ms).toBe(0);
});
