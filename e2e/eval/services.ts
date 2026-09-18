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

/**
 * The env vars this file passes through from the caller's own environment,
 * unset by default: the Jev trial's own knobs (`ORCHESTRA_PICKER` through
 * `ORCHESTRA_SERVICE_ROUTER_THRESHOLD`) and the two staging/wording flags
 * above them, each read through only when set - never a default - so a run
 * that never mentions any of them is byte-identical to before this list
 * existed. Factored out of `startEvalPlatform`'s own env object (eslint's
 * `max-lines-per-function`, harness/quality) - one name per line so a new
 * addition here is the whole diff, rather than another `? {} : {...}`
 * ternary inline.
 */
const PASSTHROUGH_ENV_VARS = [
  "ORCHESTRA_PLANNER_WORDING",
  "ORCHESTRA_PLANNER_STAGES",
  "ORCHESTRA_LLM_PROVIDER",
  "ANTHROPIC_API_KEY",
  "ORCHESTRA_PICKER",
  "ORCHESTRA_JEV_API_KEY",
  "ORCHESTRA_JEV_CRITERIA",
  "ORCHESTRA_GATE",
  "ORCHESTRA_JEV_OBJECT_INSTRUCTIONS",
  "ORCHESTRA_HYBRID_JEV_TIMEOUT",
  "ORCHESTRA_HYBRID_THRESHOLD",
  "ORCHESTRA_SERVICE_ROUTER",
  "ORCHESTRA_SERVICE_ROUTER_THRESHOLD",
  "ORCHESTRA_SERVICE_ROUTER_CRITERIA",
  "ORCHESTRA_FILL_SKIP_EMPTY",
  "ORCHESTRA_FILL_ENUM",
  "ORCHESTRA_FILL_ENUM_THRESHOLD",
  "ORCHESTRA_FILL_ENUM_REFUSAL",
  "ORCHESTRA_FILL_ENUM_UNSET_WORDING",
] as const;

/** Builds the env overrides for `PASSTHROUGH_ENV_VARS`, one entry per name actually set in `process.env`. */
function passthroughEnv(): Record<string, string> {
  const out: Record<string, string> = {};

  for (const name of PASSTHROUGH_ENV_VARS) {
    const value = process.env[name];

    if (value !== undefined) out[name] = value;
  }

  return out;
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
      // See PASSTHROUGH_ENV_VARS's own doc comment for what each of these
      // is and when it applies - every one is passed through only when
      // set in the caller's own environment, never a default.
      ...passthroughEnv(),
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
