import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import { variantSuffix } from "./flags.ts";

/**
 * `variantSuffix`'s own ORCHESTRA_PICK_WORDING tests, split into this file
 * the same way `flags_max_tokens.test.ts` is split from `flags.test.ts`
 * (eslint's `max-lines`, harness/quality). See `flags.test.ts`'s own doc
 * comment for what `variantSuffix` is.
 */

const ENV_VARS_UNDER_TEST = ["ORCHESTRA_PLANNER_MAX_TOKENS", "ORCHESTRA_PICK_WORDING"] as const;
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

test("variantSuffix appends nothing for ORCHESTRA_PICK_WORDING unset", () => {
  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends nothing for ORCHESTRA_PICK_WORDING=v1 (the default)", () => {
  process.env["ORCHESTRA_PICK_WORDING"] = "v1";

  expect(variantSuffix()).toBe("");
});

test("variantSuffix appends -pick<name> for a non-default ORCHESTRA_PICK_WORDING, after -mt", () => {
  process.env["ORCHESTRA_PLANNER_MAX_TOKENS"] = "4000";
  process.env["ORCHESTRA_PICK_WORDING"] = "v2-strict-capabilities";

  expect(variantSuffix()).toBe("-mt4000-pickv2-strict-capabilities");
});
