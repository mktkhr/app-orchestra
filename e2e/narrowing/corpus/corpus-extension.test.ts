/**
 * The extension corpus's own checks (`axis-d2.ts`, `axis-e2.ts`), mirroring
 * `corpus.test.ts`'s structural rules exactly so the extension set is held
 * to the same bar as the original 100 — a question that fails one of these
 * is not measuring the axis it claims to be in.
 *
 * Kept as its own file rather than folded into `corpus.test.ts` so the
 * original file, and the 100-question corpus it checks, is untouched.
 */
import { expect, test } from "vite-plus/test";

import { catalogOf, type FixtureOperation } from "../fixture/index.ts";
import { bigramsOf, buildIndex, combinedTextOf, scoreOperation } from "../lexical.ts";
import { extensionQuestions, questions } from "./index.ts";
import type { Question } from "./types.ts";

const CATALOG = catalogOf(5);
const OPERATIONS_BY_ID = new Map(CATALOG.map((op) => [op.operationId, op]));
const INDEX = buildIndex(CATALOG);

function operationOf(operationId: string): FixtureOperation {
  const found = OPERATIONS_BY_ID.get(operationId);

  if (found === undefined) throw new Error(`no such operation: ${operationId}`);

  return found;
}

function extensionQuestionsOf(axis: "D" | "E"): readonly Question[] {
  return extensionQuestions().filter((q) => q.axis === axis);
}

function countsByAxis(): { readonly D: number; readonly E: number } {
  const counts = { D: 0, E: 0 };

  for (const q of extensionQuestions()) {
    if (q.axis === "D") counts.D += 1;
    if (q.axis === "E") counts.E += 1;
  }

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

/** Every answer id (across the extension questions) that is not in the fixture's catalogue. */
function unknownAnswerIds(): readonly string[] {
  return extensionQuestions().flatMap((q) => q.answers.filter((a) => !OPERATIONS_BY_ID.has(a)));
}

/** Every axis E2 decoy id that is missing or not in the fixture's catalogue. */
function unknownOrMissingDecoyIds(): readonly (string | undefined)[] {
  return extensionQuestionsOf("E")
    .map((q) => q.decoy)
    .filter((decoy) => decoy === undefined || !OPERATIONS_BY_ID.has(decoy));
}

/** Whether any answer of `question` shares a bigram with `question.text`. */
function axisDQuestionOverlapsAnAnswer(question: Question): boolean {
  const questionBigrams = bigramsOf(question.text);

  return question.answers.some((answerId) => {
    const answerBigrams = bigramsOf(combinedTextOf(operationOf(answerId)));

    return [...questionBigrams].some((bigram) => answerBigrams.has(bigram));
  });
}

/** Axis D2 question ids that share a bigram with one of their answers. */
function axisD2QuestionsWithVocabularyOverlap(): readonly string[] {
  return extensionQuestionsOf("D")
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

/** Axis E2 question ids whose decoy does not out-score every answer. */
function axisE2QuestionsWithoutAnOutscoringDecoy(): readonly string[] {
  return extensionQuestionsOf("E")
    .filter((q) => axisEDecoyFailsToOutscore(q))
    .map((q) => q.id);
}

/** Whether `question`'s decoy is missing, or is not a settings/master-data operation. */
function axisEDecoyIsNotASettingsOperation(question: Question): boolean {
  return question.decoy === undefined || !operationOf(question.decoy).isSetting;
}

/** Axis E2 question ids whose decoy is missing or not a settings operation. */
function axisE2QuestionsWithoutASettingsDecoy(): readonly string[] {
  return extensionQuestionsOf("E")
    .filter((q) => axisEDecoyIsNotASettingsOperation(q))
    .map((q) => q.id);
}

/** Whether any answer of `question` is itself a settings/master-data operation. */
function axisEQuestionHasASettingsAnswer(question: Question): boolean {
  return question.answers.some((answerId) => operationOf(answerId).isSetting);
}

/** Axis E2 question ids where an answer is itself a settings operation. */
function axisE2QuestionsWithASettingsAnswer(): readonly string[] {
  return extensionQuestionsOf("E")
    .filter((q) => axisEQuestionHasASettingsAnswer(q))
    .map((q) => q.id);
}

/** Extension question ids that share a substring relationship with one of their answers' text. */
function extensionQuestionsSharingSubstringWithAnAnswer(): readonly string[] {
  return extensionQuestions()
    .filter((q) => q.answers.some((answerId) => sharesSubstringWith(q.text, operationOf(answerId))))
    .map((q) => q.id);
}

/** Extension question ids that collide with an id already used by the 100-question corpus. */
function extensionIdsCollidingWithMainCorpus(): readonly string[] {
  const mainIds = new Set(questions().map((q) => q.id));

  return extensionQuestions()
    .map((q) => q.id)
    .filter((id) => mainIds.has(id));
}

test("there are 50 extension questions", () => {
  expect(extensionQuestions().length).toBe(50);
});

test("the extension split is 25/25", () => {
  expect(countsByAxis()).toEqual({ D: 25, E: 25 });
});

test("extension question ids are unique", () => {
  const ids = extensionQuestions().map((q) => q.id);

  expect(new Set(ids).size).toBe(ids.length);
});

test("extension question ids do not collide with the main corpus", () => {
  expect(extensionIdsCollidingWithMainCorpus()).toEqual([]);
});

test("every extension answer names an operation the fixture serves", () => {
  expect(unknownAnswerIds()).toEqual([]);
});

test("every axis E2 decoy names an operation the fixture serves", () => {
  expect(unknownOrMissingDecoyIds()).toEqual([]);
});

test("every extension question has at least one answer", () => {
  const empty = extensionQuestions().filter((q) => q.answers.length === 0);

  expect(empty).toEqual([]);
});

test("axis D2 questions share no character bigram with any answer's combined text", () => {
  expect(axisD2QuestionsWithVocabularyOverlap()).toEqual([]);
});

test("axis E2 decoys out-score every answer", () => {
  expect(axisE2QuestionsWithoutAnOutscoringDecoy()).toEqual([]);
});

test("axis E2 decoys are settings operations", () => {
  expect(axisE2QuestionsWithoutASettingsDecoy()).toEqual([]);
});

test("axis E2 answers are never settings operations", () => {
  expect(axisE2QuestionsWithASettingsAnswer()).toEqual([]);
});

test("no extension question is a substring of its answer's text, and vice versa", () => {
  expect(extensionQuestionsSharingSubstringWithAnAnswer()).toEqual([]);
});

test("the lexical index built over the full catalogue is used consistently", () => {
  expect(INDEX.size).toBe(CATALOG.length);
});
