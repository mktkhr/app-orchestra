import { defineConfig } from "vite-plus";

/**
 * End-to-end tests start the built backend binary serving the built frontend
 * and talk to it over real TCP. They need `make build` to have run first.
 */
export default defineConfig({
  test: {
    environment: "node",
    // The narrowing fixture's own tests need nothing built and nothing
    // running: they assert the definition table and the OpenAPI documents
    // synthesised from it in memory (docs/plans/narrowing.md Task 1, Step 0).
    //
    // shortlist/score.test.ts and shortlist/report.test.ts are the same
    // shape - fakes only, no server, no model (docs/plans/shortlisting.md
    // Task 4). shortlist/boot.ts and shortlist/run.ts need a live platform
    // and a live model and are never imported by a *.test.ts file, so this
    // glob never pulls them into `make check`.
    //
    // eval/match.test.ts is the same shape again: match.ts's own logic
    // against fake PlanOutcomes, no server, no model. eval/run.ts,
    // plan-client.ts and services.ts (the ones that do need a live platform
    // and ORCHESTRA_EVAL_MODEL) are never *.test.ts, so `make check` still
    // calls no model (AC-E-202) even with eval/**/*.test.ts included here.
    include: [
      "src/**/*.test.ts",
      "narrowing/**/*.test.ts",
      "shortlist/**/*.test.ts",
      "eval/**/*.test.ts",
    ],
    globals: false,
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
