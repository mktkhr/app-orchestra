import { createWriteStream, mkdtempSync } from "node:fs";
import { createConnection } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";

import { start as startFixture, type Serving } from "../narrowing/serve.ts";
import { midServices, services } from "../narrowing/fixture/index.ts";
import {
  freePort,
  startBinary,
  stop,
  waitForReady,
  type RunningService,
} from "../src/helpers/process.ts";

/**
 * Boots the fixture server and the platform binary for the shortlist
 * measurement (docs/plans/shortlisting.md Task 4, Step 1;
 * docs/specs/shortlisting.md section 6). NOT imported by any test file:
 * it needs a live server and, for the "on" boot, a live embedder and
 * reranker - `make check` never touches this file.
 *
 * NEVER boot this alongside the real services (services/inventory,
 * services/attendance, ...) - the fixture and the real services declare
 * overlapping operation ids, and ORCHESTRA_SERVICES does not de-duplicate.
 * This file only ever starts the fixture.
 */

const ADMIN_PASSWORD = "shortlist-eval-admin-password";
const LLM_BASE_URL = "http://localhost:11435/v1";
const LLM_MODEL = "qwen3.5-9b-q8";

/**
 * The fixed "today" every shortlist measurement run pins the platform's
 * planners to (`ORCHESTRA_PLANNER_TODAY`, config.go) - 2026-09-16, the
 * date the corpus reference 78/79 was taken with in the prompt. Without
 * this, every planning call's user content starts with 「今日は
 * YYYY-MM-DD（曜）です。」 off the real clock, so a run on a different
 * calendar day is a different request and a near-tie row (such as
 * real-attendance-detail) can move on its own.
 */
const PLANNER_TODAY = "2026-09-16";

/** Narrowing on, K=20 - the shortlist measurement's own setting (docs/plans/shortlisting.md Task 4), shared by run.ts's plain/wording passes and run-mid.ts's mid pass (docs/plans/midsizing.md Task 3) so the two never drift apart. */
export const NARROWING: NarrowingOptions = {
  embedModel: "e5-large-q8",
  rerankModel: "bge-reranker-v2-m3-q8",
  k: 20,
};

export interface NarrowingOptions {
  readonly embedModel: string;
  readonly rerankModel: string;
  readonly k: number;
}

export interface BootOptions {
  /** When set, narrowing is configured on the platform; when absent, none of the three vars are set (config.go: all or none). */
  readonly narrowing?: NarrowingOptions;
  /** When set, `ORCHESTRA_PLANNER_WORDING` is set to this name on the platform (docs/plans/wording.md Task 2) - the whole contract with the `wording` package is this variable's name. */
  readonly wording?: string;
  /** When set, `ORCHESTRA_PLANNER_THINKING` is set to this on the platform (config.go: "on" or "off"; unset leaves thinking off, the default since 2026-09-16). */
  readonly thinking?: "on" | "off";
  /** When set, `ORCHESTRA_PLANNER_REPEAT_PENALTY` is set to this on the platform; unset sends nothing. */
  readonly repeatPenalty?: number;
  /** When set, `ORCHESTRA_PLANNER_STAGES` is set to this on the platform (docs/plans/staging.md, Task 3); unset leaves the platform's own default (2, the pick-then-fill path, the default since 2026-09-16). */
  readonly stages?: 1 | 2;
  /** When set, the platform's stdout and stderr - its JSON logs, including the `planner truncated by max_tokens` warn line - are written to this file instead of being discarded. */
  readonly logFile?: string;
  /** Which fixture subset to serve: the full five-service fixture (default), or the mid subset's three services (docs/plans/midsizing.md Task 3) - `midServices()`, thirty operations. */
  readonly fixture?: "full" | "mid";
}

export interface Booted {
  readonly baseUrl: string;
  readonly adminPassword: string;
  readonly stop: () => Promise<void>;
}

let fixture: Serving | undefined;
let platform: RunningService | undefined;
let usedPorts: readonly number[] = [];

/** Whether something is listening on port right now, by attempting a real TCP connect. */
function isListening(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = createConnection({ port, host: "127.0.0.1" });

    socket.once("connect", () => {
      socket.destroy();
      resolve(true);
    });
    socket.once("error", () => {
      resolve(false);
    });
  });
}

/**
 * Verifies nothing is left listening on any port this boot used. This
 * repository has a documented history of leftover zombie processes eating
 * memory (e2e/eval/services.ts and its own history), so this throws loudly
 * rather than logging a warning.
 */
async function verifyPortsFree(ports: readonly number[]): Promise<void> {
  const stillListening: number[] = [];

  for (const port of ports) {
    // Sequential and deliberate: a handful of ports, checked once, right
    // after teardown - not a load test.
    if (await isListening(port)) stillListening.push(port);
  }

  if (stillListening.length > 0) {
    throw new Error(
      `shortlist boot: still listening after teardown on port(s) ${stillListening.join(", ")}`,
    );
  }
}

/**
 * Starts the fixture server and the platform binary on free ports, with
 * narrowing on or off per `options.narrowing`, and waits for
 * `/api/health`. `stop()` kills both, then port-probes every port this
 * boot used and throws if any is still listening.
 */
export async function boot(options: BootOptions = {}): Promise<Booted> {
  const [fixturePort, platformPort] = await Promise.all([freePort(), freePort()]);

  usedPorts = [fixturePort, platformPort];

  const servedServices = options.fixture === "mid" ? midServices() : services();

  fixture = await startFixture(fixturePort, servedServices);

  const serviceEnv = servedServices
    .map((service) => `${service.name}=http://127.0.0.1:${String(fixturePort)}/${service.name}`)
    .join(",");

  const dbPath = path.join(
    mkdtempSync(path.join(tmpdir(), "orchestra-shortlist-")),
    "workspaces.db",
  );

  platform = {
    process: startBinary("../services/platform/bin/api", {
      ORCHESTRA_PORT: String(platformPort),
      ORCHESTRA_SERVICES: serviceEnv,
      ORCHESTRA_DB_PATH: dbPath,
      ORCHESTRA_ADMIN_PASSWORD: ADMIN_PASSWORD,
      ORCHESTRA_LLM_BASE_URL: LLM_BASE_URL,
      ORCHESTRA_LLM_MODEL: LLM_MODEL,
      ORCHESTRA_SECURE_COOKIE: "false",
      ORCHESTRA_PLANNER_TODAY: PLANNER_TODAY,
      ...(options.narrowing === undefined
        ? {}
        : {
            ORCHESTRA_NARROWING_EMBED_MODEL: options.narrowing.embedModel,
            ORCHESTRA_NARROWING_RERANK_MODEL: options.narrowing.rerankModel,
            ORCHESTRA_NARROWING_K: String(options.narrowing.k),
          }),
      ...(options.wording === undefined ? {} : { ORCHESTRA_PLANNER_WORDING: options.wording }),
      ...(options.thinking === undefined ? {} : { ORCHESTRA_PLANNER_THINKING: options.thinking }),
      ...(options.repeatPenalty === undefined
        ? {}
        : { ORCHESTRA_PLANNER_REPEAT_PENALTY: String(options.repeatPenalty) }),
      ...(options.stages === undefined ? {} : { ORCHESTRA_PLANNER_STAGES: String(options.stages) }),
      // The Jev trial (2026-09-17): ORCHESTRA_PICKER/ORCHESTRA_JEV_API_KEY,
      // like e2e/eval/services.ts's own pair, are passed through only when
      // set in the caller's own environment - never a default - so a run
      // that never mentions either is byte-identical to before this pair
      // existed. Env vars, not BootOptions fields, the same way `make
      // eval-shortlist PICKER=jev` reaches this file through the process
      // environment rather than a run.ts flag (variantSuffix, flags.ts).
      ...(process.env["ORCHESTRA_PICKER"] === undefined
        ? {}
        : { ORCHESTRA_PICKER: process.env["ORCHESTRA_PICKER"] }),
      ...(process.env["ORCHESTRA_JEV_API_KEY"] === undefined
        ? {}
        : { ORCHESTRA_JEV_API_KEY: process.env["ORCHESTRA_JEV_API_KEY"] }),
    }),
    port: platformPort,
  };

  if (options.logFile !== undefined) {
    const stream = createWriteStream(options.logFile);

    platform.process.stdout?.pipe(stream, { end: false });
    platform.process.stderr?.pipe(stream);
  }

  await waitForReady(`http://127.0.0.1:${platformPort}/api/health`, 20_000);

  const baseUrl = `http://127.0.0.1:${platformPort}`;

  return {
    baseUrl,
    adminPassword: ADMIN_PASSWORD,
    stop: async () => {
      await Promise.all([stop(platform), fixture?.stop()]);
      await verifyPortsFree(usedPorts);
    },
  };
}
