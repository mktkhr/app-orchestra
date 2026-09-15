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
  attendance = {
    process: startBinary("../services/attendance/bin/api", {
      ORCHESTRA_PORT: String(attendancePort),
    }),
    port: attendancePort,
  };

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
    }),
    port: platformPort,
  };

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 15_000);

  const baseUrl = `http://127.0.0.1:${platformPort}`;
  const session = await signIn(baseUrl, "admin", adminPassword);

  return { baseUrl, session };
}

/** Stops every process startEvalPlatform started. */
export async function stopEvalPlatform(): Promise<void> {
  await Promise.all([stop(inventory), stop(attendance), stop(platform)]);
}
