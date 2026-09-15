import { mkdtempSync } from "node:fs";
import { createConnection } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";

import { start as startFixture, type Serving } from "../narrowing/serve.ts";
import { services } from "../narrowing/fixture/index.ts";
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
  fixture = await startFixture(fixturePort);

  const serviceEnv = services()
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
      ...(options.narrowing === undefined
        ? {}
        : {
            ORCHESTRA_NARROWING_EMBED_MODEL: options.narrowing.embedModel,
            ORCHESTRA_NARROWING_RERANK_MODEL: options.narrowing.rerankModel,
            ORCHESTRA_NARROWING_K: String(options.narrowing.k),
          }),
      ...(options.wording === undefined ? {} : { ORCHESTRA_PLANNER_WORDING: options.wording }),
    }),
    port: platformPort,
  };

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
