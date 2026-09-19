import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import { variantSuffix } from "./flags.ts";

/**
 * `variantSuffix`'s own ORCHESTRA_PLANNER_MAX_TOKENS tests, split into this
 * file rather than added to `flags.test.ts` (eslint's `max-lines`,
 * harness/quality - that file was already at its own 300-line cap). See
 * `flags.test.ts`'s own doc comment for what `variantSuffix` is.
 */

const ENV_VARS_UNDER_TEST = [
  "ORCHESTRA_LLM_MODEL",
  "ORCHESTRA_ANTHROPIC_THINKING",
  "ORCHESTRA_PLANNER_MAX_TOKENS",
] as const;
const originalEnv: Record<string, string | undefined> = {};

for (const name of ENV_VARS_UNDER_TEST) originalEnv[name] = process.env[name];

beforeEach(() => {
  for (const name of ENV_VARS_UNDER_TEST) delete process.env[name];
});

afterEach(() => {
  for (const name of ENV_VARS_UNDER_TEST) {
    delete process.env[name];
    const original = originalEnv[name];

    if (original !== undefined) process.env[name] = original;
  }
});

test("variantSuffix appends nothing for ORCHESTRA_PLANNER_MAX_TOKENS unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -mt<value> for ORCHESTRA_PLANNER_MAX_TOKENS, after -think", () => {
  process.env["ORCHESTRA_LLM_MODEL"] = "claude-sonnet-5";
  process.env["ORCHESTRA_ANTHROPIC_THINKING"] = "on";
  process.env["ORCHESTRA_PLANNER_MAX_TOKENS"] = "4000";

  expect(variantSuffix()).toBe("-model-claude-sonnet-5-think-mt4000");
});
