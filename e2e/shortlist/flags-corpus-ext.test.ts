import { expect, test } from "vite-plus/test";

import { corpusArg, missesVariant, passOutputName, wordingOutputName } from "./flags.ts";

/**
 * flags.ts's `--corpus ext` support: `corpusArg()` accepting `"ext"`, and
 * its output-naming helpers (`passOutputName`, `wordingOutputName`,
 * `missesVariant`) - the 50-question axis D/E extension corpus's own
 * output files must never collide with the 100-question corpus's own.
 * Split out of `flags.test.ts` only for eslint's `max-lines` cap
 * (harness/quality/file-length.txt) - no server, no model.
 */

test("--corpus ext parses", () => {
  const originalArgv = process.argv;

  process.argv = [...originalArgv, "--corpus", "ext"];

  try {
    expect(corpusArg()).toBe("ext");
  } finally {
    process.argv = originalArgv;
  }
});

test("passOutputName keeps the 100-question corpus's plain on/off names unprefixed", () => {
  expect(passOutputName("on", false)).toBe("on");
  expect(passOutputName("off", false)).toBe("off");
});

test("passOutputName prefixes --corpus ext's plain on/off names with ext-", () => {
  expect(passOutputName("on", true)).toBe("ext-on");
  expect(passOutputName("off", true)).toBe("ext-off");
});

test("wordingOutputName keeps the 100-question corpus's on-<variant> naming unchanged", () => {
  expect(wordingOutputName("v1", false)).toBe("on-v1");
  expect(wordingOutputName("default-stages2-model-bonsai2-27b", false)).toBe(
    "on-default-stages2-model-bonsai2-27b",
  );
});

test("wordingOutputName prefixes --corpus ext's variant naming with ext- in place of on-", () => {
  expect(wordingOutputName("v1", true)).toBe("ext-v1");
  expect(wordingOutputName("default-stages2-model-bonsai2-27b", true)).toBe(
    "ext-default-stages2-model-bonsai2-27b",
  );
});

test("missesVariant leaves the 100-question corpus's miss-list name unprefixed, unchanged", () => {
  expect(missesVariant("v1", false)).toBe("v1");
});

test("missesVariant prefixes --corpus ext's miss-list name with ext- so it never collides", () => {
  expect(missesVariant("v1", true)).toBe("ext-v1");
});
