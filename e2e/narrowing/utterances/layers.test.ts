import { expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import { mergeUtterances, writtenUtterancesOf } from "./layers.ts";

function op(operationId: string, examples: readonly string[]): FixtureOperation {
  return {
    operationId,
    service: "test",
    serviceDisplayName: "",
    summary: "",
    description: "",
    displayName: "",
    isSetting: false,
    examples,
  };
}

test("writtenUtterancesOf reads FixtureOperation.examples, keyed by operationId", () => {
  const catalog = [op("opA", ["a1", "a2"]), op("opB", [])];
  const written = writtenUtterancesOf(catalog);

  expect(written.get("opA")).toEqual(["a1", "a2"]);
});

test("writtenUtterancesOf carries an empty array through, not undefined", () => {
  const catalog = [op("opB", [])];
  const written = writtenUtterancesOf(catalog);

  expect(written.get("opB")).toEqual([]);
});

test("mergeUtterances unions both layers for an operation present in both", () => {
  const generated = new Map([["opA", ["g1", "g2"]]]);
  const written = new Map([["opA", ["w1"]]]);

  expect(mergeUtterances(generated, written).get("opA")).toEqual(["g1", "g2", "w1"]);
});

test("mergeUtterances keeps an operation present in only one layer", () => {
  const generated = new Map([["opA", ["g1"]]]);
  const written = new Map([["opB", ["w1"]]]);
  const merged = mergeUtterances(generated, written);

  expect(merged.get("opA")).toEqual(["g1"]);
  expect(merged.get("opB")).toEqual(["w1"]);
});
