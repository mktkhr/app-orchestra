import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import { corpusArg, variantSuffix } from "./flags.ts";

/**
 * flags.ts's own tests: `--corpus` parsing (docs/plans/midsizing.md Task
 * 3, AC-M-104's own step 1) and `variantSuffix`, moved here from
 * `run.ts`'s own (untested) inline function when it was split out for
 * reuse by `run-mid.ts`. No server, no model.
 */

const originalArgv = process.argv;

beforeEach(() => {
  process.argv = [...originalArgv];
});

afterEach(() => {
  process.argv = originalArgv;
});

test("--corpus is undefined when not given", () => {
  expect(corpusArg()).toBeUndefined();
});

test("--corpus mid parses", () => {
  process.argv = [...process.argv, "--corpus", "mid"];

  expect(corpusArg()).toBe("mid");
});

test("--corpus with anything else is a usage error", () => {
  process.argv = [...process.argv, "--corpus", "shortlist"];

  expect(() => corpusArg()).toThrow(/--corpus must be "mid"/u);
});

test("variantSuffix is empty for the plain pass (nothing given)", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix composes thinking, repeat penalty and stages", () => {
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2");
});

test("variantSuffix treats stages 1 as the plain pass (no suffix)", () => {
  expect(variantSuffix(undefined, undefined, 1)).toBe("");
});
