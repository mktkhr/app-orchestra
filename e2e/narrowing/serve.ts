import { createServer, type Server } from "node:http";

import { services } from "./fixture/index.ts";
import { toOpenAPI } from "./fixture/openapi.ts";

/**
 * Serves the five fixture contracts over HTTP, the way a real service
 * serves `GET /openapi.yaml` for the platform's spec source
 * (`services/platform/internal/specsource/http`) to read.
 *
 * It serves contracts only - it does not implement the operations in them
 * (docs/specs/narrowing.md section 8). There is no route here but this one.
 *
 * No YAML serialiser is used or added: every JSON document is already a
 * valid YAML document, and the platform's contract loader
 * (`github.com/getkin/kin-openapi/openapi3`) reads both, so the body is
 * `JSON.stringify` served as `application/yaml`.
 */

const DEFAULT_PORT = 8090;

/** A running fixture server and the means to stop it. */
export interface Serving {
  readonly server: Server;
  readonly stop: () => Promise<void>;
}

/** Starts the fixture server on port, resolving once it is listening. */
export function start(port: number): Promise<Serving> {
  const byName = new Map(services().map((service) => [service.name, service]));

  const server = createServer((req, res) => {
    const match = /^\/([^/]+)\/openapi\.yaml$/u.exec(req.url ?? "");
    const service = match === undefined || match === null ? undefined : byName.get(match[1] ?? "");

    if (service === undefined) {
      res.writeHead(404, { "content-type": "text/plain" });
      res.end("not found");

      return;
    }

    const body = JSON.stringify(toOpenAPI(service));

    res.writeHead(200, { "content-type": "application/yaml" });
    res.end(body);
  });

  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, () => {
      resolve({
        server,
        stop: () =>
          new Promise<void>((resolveStop, rejectStop) => {
            server.close((error) => {
              if (error) rejectStop(error);
              else resolveStop();
            });
          }),
      });
    });
  });
}

if (import.meta.url === `file://${process.argv[1] ?? ""}`) {
  const port =
    process.env["NARROWING_PORT"] === undefined
      ? DEFAULT_PORT
      : Number(process.env["NARROWING_PORT"]);

  await start(port);

  console.log(
    `narrowing fixture serving ${String(services().length)} services on port ${String(port)}`,
  );
}
