import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import { corpusArg, narrowingArg, variantSuffix } from "./flags.ts";

/**
 * flags.ts's own tests: `--corpus` parsing (docs/plans/midsizing.md Task
 * 3, AC-M-104's own step 1) and `variantSuffix`, moved here from
 * `run.ts`'s own (untested) inline function when it was split out for
 * reuse by `run-mid.ts`. No server, no model.
 */

const originalArgv = process.argv;

/** Every env var `variantSuffix` reads, snapshotted once so `afterEach` can restore each one exactly, added to as the suffix grows a new source. */
const ENV_VARS_UNDER_TEST = [
  "ORCHESTRA_PICKER",
  "ORCHESTRA_JEV_CRITERIA",
  "ORCHESTRA_GATE",
  "ORCHESTRA_SERVICE_ROUTER",
  "ORCHESTRA_SERVICE_ROUTER_CRITERIA",
  "ORCHESTRA_FILL_ENUM",
  "ORCHESTRA_FILL_SKIP_EMPTY",
  "ORCHESTRA_FILL_ENUM_REFUSAL",
  "ORCHESTRA_FILL_ENUM_UNSET_WORDING",
  "ORCHESTRA_LLM_MODEL",
] as const;
const originalEnv: Record<string, string | undefined> = {};

for (const name of ENV_VARS_UNDER_TEST) originalEnv[name] = process.env[name];

beforeEach(() => {
  process.argv = [...originalArgv];
  for (const name of ENV_VARS_UNDER_TEST) delete process.env[name];
});

afterEach(() => {
  process.argv = originalArgv;
  for (const name of ENV_VARS_UNDER_TEST) {
    delete process.env[name];
    const original = originalEnv[name];

    if (original !== undefined) process.env[name] = original;
  }
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

test("--narrowing is undefined when not given", () => {
  expect(narrowingArg()).toBeUndefined();
});

test("--narrowing on parses", () => {
  process.argv = [...process.argv, "--narrowing", "on"];

  expect(narrowingArg()).toBe("on");
});

test("--narrowing off parses", () => {
  process.argv = [...process.argv, "--narrowing", "off"];

  expect(narrowingArg()).toBe("off");
});

test("--narrowing with anything else is a usage error", () => {
  process.argv = [...process.argv, "--narrowing", "maybe"];

  expect(() => narrowingArg()).toThrow(/--narrowing must be "on" or "off"/u);
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

test("variantSuffix appends -jev when ORCHESTRA_PICKER=jev", () => {
  process.env["ORCHESTRA_PICKER"] = "jev";

  expect(variantSuffix()).toBe("-jev");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-jev");
});

test("variantSuffix appends -hybrid when ORCHESTRA_PICKER=hybrid", () => {
  process.env["ORCHESTRA_PICKER"] = "hybrid";

  expect(variantSuffix()).toBe("-hybrid");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-hybrid");
});

test("variantSuffix appends nothing for ORCHESTRA_PICKER=local", () => {
  process.env["ORCHESTRA_PICKER"] = "local";

  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends nothing when ORCHESTRA_PICKER is unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -v2 when ORCHESTRA_PICKER=jev and ORCHESTRA_JEV_CRITERIA=v2", () => {
  process.env["ORCHESTRA_PICKER"] = "jev";
  process.env["ORCHESTRA_JEV_CRITERIA"] = "v2";

  expect(variantSuffix()).toBe("-jev-v2");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-jev-v2");
});

test("variantSuffix appends nothing for ORCHESTRA_JEV_CRITERIA=v1 (the default)", () => {
  process.env["ORCHESTRA_PICKER"] = "jev";
  process.env["ORCHESTRA_JEV_CRITERIA"] = "v1";

  expect(variantSuffix()).toBe("-jev");
});

test("variantSuffix appends nothing when ORCHESTRA_JEV_CRITERIA is unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -gate when ORCHESTRA_GATE=jev", () => {
  process.env["ORCHESTRA_GATE"] = "jev";

  expect(variantSuffix()).toBe("-gate");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-gate");
});

test("variantSuffix appends -gate after -jev-v2 when all three are set", () => {
  process.env["ORCHESTRA_PICKER"] = "jev";
  process.env["ORCHESTRA_JEV_CRITERIA"] = "v2";
  process.env["ORCHESTRA_GATE"] = "jev";

  expect(variantSuffix()).toBe("-jev-v2-gate");
});

test("variantSuffix appends nothing for ORCHESTRA_GATE=none (the default)", () => {
  process.env["ORCHESTRA_GATE"] = "none";

  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends nothing when ORCHESTRA_GATE is unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -router when ORCHESTRA_SERVICE_ROUTER=jev", () => {
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "jev";

  expect(variantSuffix()).toBe("-router");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-router");
});

test("variantSuffix appends -router after -jev-v2-gate when all four are set", () => {
  process.env["ORCHESTRA_PICKER"] = "jev";
  process.env["ORCHESTRA_JEV_CRITERIA"] = "v2";
  process.env["ORCHESTRA_GATE"] = "jev";
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "jev";

  expect(variantSuffix()).toBe("-jev-v2-gate-router");
});

test("variantSuffix appends nothing for ORCHESTRA_SERVICE_ROUTER=none (the default)", () => {
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "none";

  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends nothing when ORCHESTRA_SERVICE_ROUTER is unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -ops after -router when ORCHESTRA_SERVICE_ROUTER_CRITERIA=ops", () => {
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "jev";
  process.env["ORCHESTRA_SERVICE_ROUTER_CRITERIA"] = "ops";

  expect(variantSuffix()).toBe("-router-ops");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-router-ops");
});

test("variantSuffix appends nothing for ORCHESTRA_SERVICE_ROUTER_CRITERIA=names (the default)", () => {
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "jev";
  process.env["ORCHESTRA_SERVICE_ROUTER_CRITERIA"] = "names";

  expect(variantSuffix()).toBe("-router");
});

test("variantSuffix appends nothing when ORCHESTRA_SERVICE_ROUTER_CRITERIA is unset", () => {
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "jev";

  expect(variantSuffix()).toBe("-router");
});

test("variantSuffix never collides between the two router criteria forms", () => {
  process.env["ORCHESTRA_SERVICE_ROUTER"] = "jev";
  const names = variantSuffix();

  process.env["ORCHESTRA_SERVICE_ROUTER_CRITERIA"] = "ops";
  const ops = variantSuffix();

  expect(names).not.toBe(ops);
});

test("variantSuffix appends -fillenum when ORCHESTRA_FILL_ENUM=jev", () => {
  process.env["ORCHESTRA_FILL_ENUM"] = "jev";

  expect(variantSuffix()).toBe("-fillenum");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-fillenum");
});

test("variantSuffix appends nothing for ORCHESTRA_FILL_ENUM=none (the default)", () => {
  process.env["ORCHESTRA_FILL_ENUM"] = "none";

  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends nothing when ORCHESTRA_FILL_ENUM is unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -skipempty when ORCHESTRA_FILL_SKIP_EMPTY=1", () => {
  process.env["ORCHESTRA_FILL_SKIP_EMPTY"] = "1";

  expect(variantSuffix()).toBe("-skipempty");
  expect(variantSuffix("off", 1.1, 2)).toBe("-nothink-rp1.1-stages2-skipempty");
});

test("variantSuffix appends nothing when ORCHESTRA_FILL_SKIP_EMPTY is unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends both -fillenum and -skipempty together, fillenum first", () => {
  process.env["ORCHESTRA_FILL_ENUM"] = "jev";
  process.env["ORCHESTRA_FILL_SKIP_EMPTY"] = "1";

  expect(variantSuffix()).toBe("-fillenum-skipempty");
});

test("variantSuffix appends -refusal for =1, -refusalsep for =separate, nothing when unset", () => {
  expect(variantSuffix()).toBe("");
  process.env["ORCHESTRA_FILL_ENUM_REFUSAL"] = "1";
  expect(variantSuffix()).toBe("-refusal");
  process.env["ORCHESTRA_FILL_ENUM_REFUSAL"] = "separate";
  expect(variantSuffix()).toBe("-refusalsep");
});

test("variantSuffix appends -unsetwide for =wide, nothing for narrow (the default) or unset", () => {
  expect(variantSuffix()).toBe("");
  process.env["ORCHESTRA_FILL_ENUM_UNSET_WORDING"] = "narrow";
  expect(variantSuffix()).toBe("");
  process.env["ORCHESTRA_FILL_ENUM_UNSET_WORDING"] = "wide";
  expect(variantSuffix()).toBe("-unsetwide");
});

test("variantSuffix appends -fillenum-refusal-unsetwide together, in that order", () => {
  process.env["ORCHESTRA_FILL_ENUM"] = "jev";
  process.env["ORCHESTRA_FILL_ENUM_REFUSAL"] = "1";
  process.env["ORCHESTRA_FILL_ENUM_UNSET_WORDING"] = "wide";

  expect(variantSuffix()).toBe("-fillenum-refusal-unsetwide");
});

test("variantSuffix appends nothing for ORCHESTRA_LLM_MODEL=qwen3.5-9b-q8 (the default) or unset", () => {
  expect(variantSuffix()).toBe("");
  process.env["ORCHESTRA_LLM_MODEL"] = "qwen3.5-9b-q8";
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -model-<sanitised name> for a non-default ORCHESTRA_LLM_MODEL, after -unsetwide", () => {
  process.env["ORCHESTRA_FILL_ENUM"] = "jev";
  process.env["ORCHESTRA_FILL_ENUM_UNSET_WORDING"] = "wide";
  process.env["ORCHESTRA_LLM_MODEL"] = "Qwen3 (Q8_0)!";

  expect(variantSuffix()).toBe("-fillenum-unsetwide-model-qwen3--q8-0--");
});

test("variantSuffix sanitises a Claude model id into a usable filename suffix", () => {
  process.env["ORCHESTRA_LLM_MODEL"] = "claude-sonnet-5";

  expect(variantSuffix()).toBe("-model-claude-sonnet-5");
});
