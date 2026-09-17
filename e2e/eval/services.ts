import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import { signIn, type Session } from "../src/helpers/auth.ts";
import {
  freePort,
  startBinary,
  stop,
  waitForReady,
  type RunningService,
} from "../src/helpers/process.ts";

/**
 * Drains a spawned process's stdout/stderr so neither ever backs up.
 * `startBinary` (`e2e/src/helpers/process.ts`) pipes both
 * (`stdio: ["ignore", "pipe", "pipe"]`); a piped stream nobody reads stays
 * paused, and once its OS pipe buffer (64KiB on Linux) fills, the child's
 * own write to stdout/stderr blocks - and, for the platform, so does
 * whatever request was mid-`slog` write when that happened, forever, since
 * nothing here was ever going to read the rest.
 *
 * Found 2026-09-17, `PICKER=jev make eval`: the jev picker logs one extra
 * "pick completed" line per pick (`pick_probabilities` included) beside
 * the orchestrator's own, on top of a real run's several hundred requests
 * - enough log volume, unlike the local picker's, to fill the pipe within
 * a `make eval` run and hang a request outright (an unrelated-looking
 * `UND_ERR_HEADERS_TIMEOUT` about 5 minutes in, undici's own
 * `headersTimeout` - reproduced twice, absent from a same-day control run
 * with the local picker). The discarded output was never captured by this
 * file to begin with (unlike `e2e/shortlist/boot.ts`'s own `logFile`
 * option) - draining it costs nothing this suite already had.
 */
function drainStdio(child: RunningService["process"]): void {
  child.stdout?.resume();
  child.stderr?.resume();
}

/** The real, running platform an eval run measures against, and the session to call it with. */
export interface EvalPlatform {
  readonly baseUrl: string;
  readonly session: Session;
}

let inventory: RunningService | undefined;
let attendance: RunningService | undefined;
let platform: RunningService | undefined;

const adminPassword = "eval-admin-password";

/**
 * The fixed "today" every eval run pins the platform's planners to
 * (`ORCHESTRA_PLANNER_TODAY`, config.go) - 2026-09-16, the date the eval
 * baseline (`e2e/eval/baseline.json`) and the corpus reference 78/79 were
 * both taken with in the prompt. Without this, every planning call's user
 * content starts with 「今日は YYYY-MM-DD（曜）です。」 off the real
 * clock (`toolcall.WithClock`/`jsonmode.WithClock`'s own default), so a
 * run on a different day is a different request and a near-tie row can
 * move on its own - not because the corpus changed.
 */
const PLANNER_TODAY = "2026-09-16";

/**
 * Starts both dummy services and the platform from their built binaries -
 * the same way `e2e/src/*.test.ts` do - except the platform is started with
 * `llmBaseURL`/`llmModel` set (docs/specs/eval.md, E5/section 5), so it
 * plans with the real tool-calling planner rather than the stub every other
 * e2e suite uses.
 */
export async function startEvalPlatform(
  llmBaseURL: string,
  llmModel: string,
): Promise<EvalPlatform> {
  const [inventoryPort, attendancePort, platformPort] = await Promise.all([
    freePort(),
    freePort(),
    freePort(),
  ]);

  inventory = {
    process: startBinary("../services/inventory/bin/api", {
      ORCHESTRA_PORT: String(inventoryPort),
    }),
    port: inventoryPort,
  };
  drainStdio(inventory.process);

  attendance = {
    process: startBinary("../services/attendance/bin/api", {
      ORCHESTRA_PORT: String(attendancePort),
    }),
    port: attendancePort,
  };
  drainStdio(attendance.process);

  await Promise.all([
    waitForReady(`http://127.0.0.1:${inventoryPort}/openapi.yaml`, 10_000),
    waitForReady(`http://127.0.0.1:${attendancePort}/openapi.yaml`, 10_000),
  ]);

  const dbPath = path.join(mkdtempSync(path.join(tmpdir(), "orchestra-eval-")), "workspaces.db");

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES:
        `inventory=http://127.0.0.1:${inventoryPort},` +
        `attendance=http://127.0.0.1:${attendancePort}`,
      ORCHESTRA_LLM_BASE_URL: llmBaseURL,
      ORCHESTRA_LLM_MODEL: llmModel,
      ORCHESTRA_DB_PATH: dbPath,
      ORCHESTRA_ADMIN_PASSWORD: adminPassword,
      // Plain HTTP (127.0.0.1, no TLS): see e2e/src/orchestration.test.ts's
      // own comment on this variable.
      ORCHESTRA_SECURE_COOKIE: "false",
      ORCHESTRA_PLANNER_TODAY: PLANNER_TODAY,
      // docs/specs/wording.md Q4: the wording that would become the default
      // is checked against these eighteen cases too. Passed through only
      // when set, so the default run is byte-identical to before.
      ...(process.env["ORCHESTRA_PLANNER_WORDING"] === undefined
        ? {}
        : { ORCHESTRA_PLANNER_WORDING: process.env["ORCHESTRA_PLANNER_WORDING"] }),
      // docs/plans/staging.md, Task 4: `ORCHESTRA_PLANNER_STAGES=2 make
      // eval` must be able to check capability/unanswerable/enum cases
      // under staging too. Passed through only when set, so the default
      // run is byte-identical to before (AC-S-101).
      ...(process.env["ORCHESTRA_PLANNER_STAGES"] === undefined
        ? {}
        : { ORCHESTRA_PLANNER_STAGES: process.env["ORCHESTRA_PLANNER_STAGES"] }),
      // The Jev trial (2026-09-17): ORCHESTRA_PICKER selects the picker
      // implementation stagingOptions builds under STAGES=2 ("local" the
      // default, "jev" TypeSafe's hosted API); ORCHESTRA_JEV_API_KEY is
      // its bearer token. Passed through only when set in the caller's
      // own environment - never a default - so a run that never mentions
      // either is byte-identical to before this pair existed.
      ...(process.env["ORCHESTRA_PICKER"] === undefined
        ? {}
        : { ORCHESTRA_PICKER: process.env["ORCHESTRA_PICKER"] }),
      ...(process.env["ORCHESTRA_JEV_API_KEY"] === undefined
        ? {}
        : { ORCHESTRA_JEV_API_KEY: process.env["ORCHESTRA_JEV_API_KEY"] }),
      // The Jev trial's second round (2026-09-17, "v2: richer criteria"):
      // ORCHESTRA_JEV_CRITERIA, same pass-through as ORCHESTRA_PICKER
      // above - unset means the platform's own default (v1).
      ...(process.env["ORCHESTRA_JEV_CRITERIA"] === undefined
        ? {}
        : { ORCHESTRA_JEV_CRITERIA: process.env["ORCHESTRA_JEV_CRITERIA"] }),
      // The v3 Jev trial (2026-09-17, "a noul refusal gate in front of
      // the local pick"): ORCHESTRA_GATE, same pass-through as
      // ORCHESTRA_PICKER above - unset means the platform's own default
      // (none, no gate at all).
      ...(process.env["ORCHESTRA_GATE"] === undefined
        ? {}
        : { ORCHESTRA_GATE: process.env["ORCHESTRA_GATE"] }),
      // The v5 Jev trial's own run B (2026-09-17, "isolating turns from
      // the object instructions"): ORCHESTRA_JEV_LEGACY_INSTRUCTIONS,
      // same pass-through as ORCHESTRA_PICKER above - unset means the
      // platform's own default (the v5 object instructions apply).
      ...(process.env["ORCHESTRA_JEV_LEGACY_INSTRUCTIONS"] === undefined
        ? {}
        : { ORCHESTRA_JEV_LEGACY_INSTRUCTIONS: process.env["ORCHESTRA_JEV_LEGACY_INSTRUCTIONS"] }),
    }),
    port: platformPort,
  };
  drainStdio(platform.process);

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 15_000);

  const baseUrl = `http://127.0.0.1:${platformPort}`;
  const session = await signIn(baseUrl, "admin", adminPassword);

  return { baseUrl, session };
}

/** Stops every process startEvalPlatform started. */
export async function stopEvalPlatform(): Promise<void> {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
}
