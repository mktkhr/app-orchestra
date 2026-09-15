import { describe, expect, it } from "vite-plus/test";

import type { Turn } from "../model/turn";
import { nearestQuestion } from "./nearestQuestion";

const NONE_RESULT = { kind: "none", message: "結果はありません。" } as const;

/**
 * `nearestQuestion` walks backward from an answer to the question it
 * belongs to - the text a chip under that answer resends
 * (docs/specs/shortlisting.md, section 4). A choice turn (added for a chip
 * click, `TurnList.tsx`) must not stop that walk: the answer after it still
 * belongs to the same original question, so a chip under *that* answer
 * resends the same text, not the chosen alternative's label.
 */
describe("nearestQuestion", () => {
  it("returns the question for the answer right after it", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "勤怠記録を見せて" },
      { id: "2", role: "answer", result: NONE_RESULT },
    ];

    expect(nearestQuestion(turns, 1)).toBe("勤怠記録を見せて");
  });

  it("returns the original question for the answer after a choice turn", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "勤怠記録を見せて" },
      { id: "2", role: "answer", result: NONE_RESULT },
      { id: "3", role: "choice", text: "勤怠記録を見せて", label: "勤怠記録の詳細" },
      { id: "4", role: "answer", result: NONE_RESULT },
    ];

    expect(nearestQuestion(turns, 3)).toBe("勤怠記録を見せて");
  });

  it("returns empty text when no question precedes the index", () => {
    const turns: Turn[] = [{ id: "1", role: "answer", result: NONE_RESULT }];

    expect(nearestQuestion(turns, 0)).toBe("");
  });
});
