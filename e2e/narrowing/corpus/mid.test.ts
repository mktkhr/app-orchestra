/**
 * The mid corpus's own checks (docs/plans/midsizing.md Task 2 Step 1,
 * AC-M-103). Task 1 (`e2e/narrowing/fixture/mid.ts`, `midCatalog()`) has not
 * landed yet, so the thirty operation ids are computed here from the same
 * naming rule the fixture generator uses (`fixture/naming.ts`'s `ALL_VERBS`,
 * `pluralOf`, `qualify`) rather than imported.
 *
 * TODO(midsizing Task 3): switch to `midCatalog()` once Task 1 is merged,
 * and drop `MID_RESOURCES` / `midOperationIds` below.
 */
import { expect, test } from "vite-plus/test";

import { catalogOf, type FixtureOperation } from "../fixture/index.ts";
import { ALL_VERBS, pluralOf, qualify } from "../fixture/naming.ts";
import { midQuestions } from "./mid.ts";
import type { MidQuestion } from "./types.ts";

/** docs/specs/midsizing.md M1: three services, two resources each. */
const MID_RESOURCES: Readonly<Record<string, readonly string[]>> = {
  sales: ["Order", "Partner"],
  purchasing: ["Order", "Partner"],
  attendance: ["Employee", "LeaveRequest"],
};

/** The five operation ids one resource generates, by the fixture's naming rule. */
function operationIdsOfResource(service: string, resource: string): readonly string[] {
  return ALL_VERBS.map((verb) => {
    const idPart = verb === "list" ? pluralOf(resource) : resource;

    return `${verb}${qualify(service, idPart)}`;
  });
}

/** The thirty mid operation ids, computed rather than imported (see header). */
function midOperationIds(): readonly string[] {
  return Object.entries(MID_RESOURCES).flatMap(([service, resources]) =>
    resources.flatMap((resource) => operationIdsOfResource(service, resource)),
  );
}

const MID_OPERATION_IDS = midOperationIds();
const MID_OPERATION_ID_SET = new Set(MID_OPERATION_IDS);

const FULL_CATALOG = catalogOf(5);
const MID_OPERATIONS_BY_ID = new Map(
  FULL_CATALOG.filter((op) => MID_OPERATION_ID_SET.has(op.operationId)).map((op) => [
    op.operationId,
    op,
  ]),
);

function answerable(): readonly MidQuestion[] {
  return midQuestions().filter((q) => q.expect === "answerable");
}

function impossible(): readonly MidQuestion[] {
  return midQuestions().filter((q) => q.expect === "impossible");
}

/** Answerable question ids whose `answers` is empty or names an id outside the subset. */
function answerableQuestionsWithBadAnswers(): readonly string[] {
  return answerable()
    .filter((q) => q.answers.length === 0 || q.answers.some((a) => !MID_OPERATION_ID_SET.has(a)))
    .map((q) => q.id);
}

/** Every mid operation id that no answerable question names. */
function operationsNamedByNoQuestion(): readonly string[] {
  const named = new Set(answerable().flatMap((q) => q.answers));

  return MID_OPERATION_IDS.filter((id) => !named.has(id));
}

/** Impossible question ids whose `answers` is non-empty. */
function impossibleQuestionsWithAnswers(): readonly string[] {
  return impossible()
    .filter((q) => q.answers.length > 0)
    .map((q) => q.id);
}

/** Question ids sharing their `text` with another question. */
function questionsWithDuplicateText(): readonly string[] {
  const seen = new Map<string, number>();

  for (const q of midQuestions()) seen.set(q.text, (seen.get(q.text) ?? 0) + 1);

  return midQuestions()
    .filter((q) => (seen.get(q.text) ?? 0) > 1)
    .map((q) => q.id);
}

/** Every text field of an operation the blindness check compares a question against. */
function textFieldsOf(operation: FixtureOperation): readonly string[] {
  return [operation.summary, operation.description, operation.displayName];
}

/** Answerable question ids whose text equals one of their answer's summary/description/displayName. */
function answerableQuestionsMatchingAnOperationSummary(): readonly string[] {
  return answerable()
    .filter((q) =>
      q.answers.some((answerId) => {
        const operation = MID_OPERATIONS_BY_ID.get(answerId);

        return operation !== undefined && textFieldsOf(operation).includes(q.text);
      }),
    )
    .map((q) => q.id);
}

test("there are 60 questions", () => {
  expect(midQuestions().length).toBe(60);
});

test("the split is 40 answerable / 20 impossible", () => {
  expect([answerable().length, impossible().length]).toEqual([40, 20]);
});

test("question ids are unique and run m01..m60", () => {
  const ids = midQuestions().map((q) => q.id);
  const expected = Array.from({ length: 60 }, (_, i) => `m${String(i + 1).padStart(2, "0")}`);

  expect(new Set(ids).size).toBe(60);
  expect(ids.toSorted()).toEqual(expected);
});

test("the computed catalogue has exactly the thirty mid operations", () => {
  expect(MID_OPERATION_IDS.length).toBe(30);
  expect(MID_OPERATIONS_BY_ID.size).toBe(30);
});

test("every answerable question has a non-empty answers list naming subset operations", () => {
  expect(answerableQuestionsWithBadAnswers()).toEqual([]);
});

test("every one of the thirty operations is named by at least one answerable question", () => {
  expect(operationsNamedByNoQuestion()).toEqual([]);
});

test("every impossible question has an empty answers list", () => {
  expect(impossibleQuestionsWithAnswers()).toEqual([]);
});

test("exactly five impossible questions are marked capability", () => {
  expect(impossible().filter((q) => q.capability === true).length).toBe(5);
});

test("no capability flag appears on an answerable question", () => {
  expect(answerable().filter((q) => q.capability === true).length).toBe(0);
});

test("no question text is duplicated", () => {
  expect(questionsWithDuplicateText()).toEqual([]);
});

test("no answerable question restates an answer's summary, description or display name", () => {
  expect(answerableQuestionsMatchingAnOperationSummary()).toEqual([]);
});

test("at least thirteen answerable questions carry an argument (an id, a name, or a quantity)", () => {
  const ARGUMENT_PATTERN = /(so|po|att)-\d+|名前は|\d+日間/u;
  const withArgument = answerable().filter((q) => ARGUMENT_PATTERN.test(q.text));

  expect(withArgument.length >= 13).toBe(true);
});
