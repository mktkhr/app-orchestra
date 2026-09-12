import { type ChildProcess, spawn } from "node:child_process";
import { createServer } from "node:net";
import { setTimeout as delay } from "node:timers/promises";

/**
 * Starting, stopping and waiting on a built binary over real TCP, shared by
 * every process-level e2e suite (`e2e/src/*.test.ts`). Each of those used
 * to carry its own copy of these four functions - small enough on their
 * own that `orchestration.test.ts` and `workspaces.test.ts` once argued
 * for keeping the duplication (see `workspaces.test.ts`'s own history) -
 * but a third file (`auth.test.ts`) repeating them made the trade-off go
 * the other way, and pushed `auth.test.ts` itself over the 300-line lint
 * ceiling on top of that.
 */

/** A binary started by {@link startBinary}, and the free port it was told to listen on. */
export interface RunningService {
  readonly process: ChildProcess;
  readonly port: number;
}

/** A TCP port nothing is listening on right now. */
export function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer();

    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();

      if (address === null || typeof address === "string") {
        reject(new Error("could not read the allocated port"));

        return;
      }

      const { port } = address;

      server.close(() => {
        resolve(port);
      });
    });
  });
}

/** Polls url until it answers with a 2xx status, or gives up after timeoutMs. */
export async function waitForReady(url: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  for (;;) {
    try {
      const response = await fetch(url);

      if (response.ok) return;
    } catch {
      // Not listening yet - keep polling.
    }

    if (Date.now() > deadline) {
      throw new Error(`${url} did not become ready within ${timeoutMs}ms`);
    }

    await delay(200);
  }
}

/** Starts a built binary with the given env merged over the current one. */
export function startBinary(command: string, env: Record<string, string>): ChildProcess {
  const child = spawn(command, [], {
    cwd: new URL("../..", import.meta.url).pathname,
    env: { ...process.env, ...env },
    stdio: ["ignore", "pipe", "pipe"],
  });

  child.on("error", (error) => {
    throw error;
  });

  return child;
}

/** Kills service's process and waits for it to actually exit. */
export async function stop(service: RunningService | undefined): Promise<void> {
  if (service === undefined) return;

  service.process.kill();

  await new Promise<void>((resolve) => {
    service.process.once("exit", () => {
      resolve();
    });
  });
}
