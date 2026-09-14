/**
 * The corpus's own checks (docs/plans/narrowing.md Task 4, Step 2). These are
 * what make the five axes real: a question that fails one of these is not
 * measuring the axis it claims to be in, and is rewritten rather than
 * excused (spec section 4, plan Task 4 Step 3).
 *
 * Every check below is a plain function that returns the list of offending
 * question ids (empty when everything passes); the `test` bodies only call
 * one and assert it is empty. `vitest/no-conditional-in-test` forbids a
 * conditional inside a test body, so the conditionals that decide "is this
 * question bad" all live in these helpers instead.
 */
import { expect, test } from "vite-plus/test";

import { catalogOf, type FixtureOperation } from "../fixture/index.ts";
import { bigramsOf, buildIndex, combinedTextOf, scoreOperation } from "../lexical.ts";
import { questions } from "./index.ts";
import type { Axis, Question } from "./types.ts";

const CATALOG = catalogOf(5);
const OPERATIONS_BY_ID = new Map(CATALOG.map((op) => [op.operationId, op]));
const INDEX = buildIndex(CATALOG);

function operationOf(operationId: string): FixtureOperation {
  const found = OPERATIONS_BY_ID.get(operationId);

  if (found === undefined) throw new Error(`no such operation: ${operationId}`);

  return found;
}

function questionsOf(axis: Axis): readonly Question[] {
  return questions().filter((q) => q.axis === axis);
}

function countsByAxis(): Readonly<Record<Axis, number>> {
  const counts: Record<Axis, number> = { A: 0, B: 0, C: 0, D: 0, E: 0 };

  for (const q of questions()) counts[q.axis] += 1;

  return counts;
}

/** Every field of an operation's text that the substring rule applies to. */
function textFieldsOf(operation: FixtureOperation): readonly string[] {
  return [operation.summary, operation.description, operation.displayName];
}

/** Whether `question` and any text field of `operation` share a substring relationship. */
function sharesSubstringWith(question: string, operation: FixtureOperation): boolean {
  return textFieldsOf(operation).some(
    (field) => field.includes(question) || question.includes(field),
  );
}

/** The distinct services named by a set of operationIds. */
function servicesOf(operationIds: readonly string[]): ReadonlySet<string> {
  return new Set(operationIds.map((id) => operationOf(id).service));
}

/** Every answer id (across all questions) that is not in the fixture's catalogue. */
function unknownAnswerIds(): readonly string[] {
  return questions().flatMap((q) => q.answers.filter((a) => !OPERATIONS_BY_ID.has(a)));
}

/** Every axis E decoy id that is missing or not in the fixture's catalogue. */
function unknownOrMissingDecoyIds(): readonly (string | undefined)[] {
  return questionsOf("E")
    .map((q) => q.decoy)
    .filter((decoy) => decoy === undefined || !OPERATIONS_BY_ID.has(decoy));
}

/** Axis B question ids whose answers do not span two or more services. */
function axisBQuestionsMissingCrossServiceAnswers(): readonly string[] {
  return questionsOf("B")
    .filter((q) => servicesOf(q.answers).size < 2)
    .map((q) => q.id);
}

/** Axis C question ids that do not have two or more answers in one service. */
function axisCQuestionsMissingSameServiceAnswers(): readonly string[] {
  return questionsOf("C")
    .filter((q) => q.answers.length < 2 || servicesOf(q.answers).size !== 1)
    .map((q) => q.id);
}

/** Whether any answer of `question` shares a bigram with `question.text`. */
function axisDQuestionOverlapsAnAnswer(question: Question): boolean {
  const questionBigrams = bigramsOf(question.text);

  return question.answers.some((answerId) => {
    const answerBigrams = bigramsOf(combinedTextOf(operationOf(answerId)));

    return [...questionBigrams].some((bigram) => answerBigrams.has(bigram));
  });
}

/** Axis D question ids that share a bigram with one of their answers. */
function axisDQuestionsWithVocabularyOverlap(): readonly string[] {
  return questionsOf("D")
    .filter((q) => axisDQuestionOverlapsAnAnswer(q))
    .map((q) => q.id);
}

/** The decoy's score for `question`, or -1 when there is no decoy to score. */
function decoyScoreOf(question: Question): number {
  return question.decoy === undefined
    ? -1
    : scoreOperation(operationOf(question.decoy), question.text);
}

/** Whether `question`'s decoy fails to out-score every one of its answers. */
function axisEDecoyFailsToOutscore(question: Question): boolean {
  const decoyScore = decoyScoreOf(question);

  return question.answers.some(
    (answerId) => scoreOperation(operationOf(answerId), question.text) >= decoyScore,
  );
}

/** Axis E question ids whose decoy does not out-score every answer. */
function axisEQuestionsWithoutAnOutscoringDecoy(): readonly string[] {
  return questionsOf("E")
    .filter((q) => axisEDecoyFailsToOutscore(q))
    .map((q) => q.id);
}

/**
 * Whether `question`'s decoy is missing, or is not a settings/master-data
 * operation — read structurally off `FixtureOperation.isSetting`
 * (`fixture/index.ts`, set from the operation's OpenAPI `tags`), not
 * guessed from the operationId's spelling.
 */
function axisEDecoyIsNotASettingsOperation(question: Question): boolean {
  return question.decoy === undefined || !operationOf(question.decoy).isSetting;
}

/** Axis E question ids whose decoy is missing or not a settings operation. */
function axisEQuestionsWithoutASettingsDecoy(): readonly string[] {
  return questionsOf("E")
    .filter((q) => axisEDecoyIsNotASettingsOperation(q))
    .map((q) => q.id);
}

/**
 * Whether any answer of `question` is itself a settings/master-data
 * operation. A person asking about a transaction never wants the setting
 * that configures it; a question whose answer IS the settings operation is
 * not measuring axis E, it is measuring nothing — `corpus.test.ts` was
 * fooled by nine of these before this check existed.
 */
function axisEQuestionHasASettingsAnswer(question: Question): boolean {
  return question.answers.some((answerId) => operationOf(answerId).isSetting);
}

/** Axis E question ids where an answer is itself a settings operation. */
function axisEQuestionsWithASettingsAnswer(): readonly string[] {
  return questionsOf("E")
    .filter((q) => axisEQuestionHasASettingsAnswer(q))
    .map((q) => q.id);
}

/** Question ids that share a substring relationship with one of their answers' text. */
function questionsSharingSubstringWithAnAnswer(): readonly string[] {
  return questions()
    .filter((q) => q.answers.some((answerId) => sharesSubstringWith(q.text, operationOf(answerId))))
    .map((q) => q.id);
}

test("there are 100 questions", () => {
  expect(questions().length).toBe(100);
});

test("the split is 25/25/25/15/10", () => {
  expect(countsByAxis()).toEqual({ A: 25, B: 25, C: 25, D: 15, E: 10 });
});

test("question ids are unique", () => {
  const ids = questions().map((q) => q.id);

  expect(new Set(ids).size).toBe(ids.length);
});

test("every answer names an operation the fixture serves", () => {
  expect(unknownAnswerIds()).toEqual([]);
});

test("every axis E decoy names an operation the fixture serves", () => {
  expect(unknownOrMissingDecoyIds()).toEqual([]);
});

test("every question has at least one answer", () => {
  const empty = questions().filter((q) => q.answers.length === 0);

  expect(empty).toEqual([]);
});

test("axis B questions have two or more answers in different services", () => {
  expect(axisBQuestionsMissingCrossServiceAnswers()).toEqual([]);
});

test("axis C questions have two or more answers in the same service", () => {
  expect(axisCQuestionsMissingSameServiceAnswers()).toEqual([]);
});

test("axis D questions share no character bigram with any answer's combined text", () => {
  expect(axisDQuestionsWithVocabularyOverlap()).toEqual([]);
});

test("axis E decoys out-score every answer", () => {
  expect(axisEQuestionsWithoutAnOutscoringDecoy()).toEqual([]);
});

test("axis E decoys are settings operations", () => {
  expect(axisEQuestionsWithoutASettingsDecoy()).toEqual([]);
});

test("axis E answers are never settings operations", () => {
  expect(axisEQuestionsWithASettingsAnswer()).toEqual([]);
});

test("no question is a substring of its answer's text, and no answer's text is a substring of the question", () => {
  expect(questionsSharingSubstringWithAnAnswer()).toEqual([]);
});

test("the lexical index built over the full catalogue is used consistently", () => {
  expect(INDEX.size).toBe(CATALOG.length);
});
