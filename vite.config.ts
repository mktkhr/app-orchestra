import { defineConfig } from "vite-plus";

import { fmtPolicy } from "./harness/quality/oxfmt/policy.ts";
import { lintPolicy } from "./harness/quality/oxlint/policy.ts";

/**
 * Workspace root configuration for Vite+.
 *
 * Only cross-cutting policy lives here (format, lint, type check, staged
 * checks). Application concerns (dev server, build, tests) live in each
 * package's own vite.config.ts. The policies themselves are defined under
 * harness/quality/ so that the Repository Harness is one reviewable unit.
 */
export default defineConfig({
  fmt: fmtPolicy,
  lint: lintPolicy,
  test: {
    // Unit tests of the repository tooling itself. Package tests live in each package.
    include: ["harness/**/*.test.ts"],
    environment: "node",
  },
  staged: {
    "*.{ts,tsx,js,mjs,cjs,json,css,md,yml,yaml}": "vp check --fix",
  },
});
