import { createServer, type Server } from "node:http";

import { isRecord } from "../src/helpers/wire.ts";
import { services } from "./fixture/index.ts";
import { toOpenAPI } from "./fixture/openapi.ts";
import type { Operation, PathItem, SchemaObject } from "./fixture/openapi.ts";

/**
 * Serves the five fixture contracts over HTTP, the way a real service
 * serves `GET /openapi.yaml` for the platform's spec source
 * (`services/platform/internal/specsource/http`) to read.
 *
 * It also answers `GET` on the invoke paths those contracts declare
 * (docs/specs/narrowing.md section 8 excludes only unsafe operations from
 * reaching this fixture - a safe `list`/`get`/`search`/`summarize`/
 * `aggregate`/settings-read call is not one of those, and now gets a 200
 * whose body validates against the operation's own declared response
 * schema, rather than a 404 the platform turns into a fake 500). Every
 * other method - `POST`, `PUT`, `DELETE` - still 404s: an unsafe operation
 * never reaches a service (D8), so this fixture never needs to answer one.
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

type Schemas = Readonly<Record<string, SchemaObject>>;

/** The schema definitions object a `$ref` like `#/components/schemas/Foo` points into. */
function resolveSchema(schemas: Schemas, ref: string): SchemaObject {
  const name = ref.slice(ref.lastIndexOf("/") + 1);
  const schema = schemas[name];

  if (schema === undefined) throw new Error(`fixture body: unknown schema ${ref}`);

  return schema;
}

/**
 * A minimal value satisfying one schema, driven entirely by the schema
 * itself (not by guessing field names): an object gets its required
 * properties only, each filled minimally in turn; an enum gets its first
 * value, which is always valid.
 */
function minimalOf(schemas: Schemas, schema: SchemaObject): unknown {
  if ("enum" in schema) return schema.enum[0];

  const body: Record<string, unknown> = {};

  for (const key of schema.required) body[key] = minimalProperty(schemas, schema.properties[key]);

  return body;
}

/** A minimal value for one object property: a resolved `$ref`, an empty array, or a placeholder string. */
function minimalProperty(schemas: Schemas, property: unknown): unknown {
  if (!isRecord(property)) return "fixture";

  const ref = property["$ref"];

  if (typeof ref === "string") return minimalOf(schemas, resolveSchema(schemas, ref));
  if (property["type"] === "array") return [];

  return "fixture";
}

/** The body a GET operation's 200 response declares, minimally filled - `{}` when it declares no schema. */
function fixtureBody(schemas: Schemas, operation: Operation): unknown {
  const content = operation.responses["200"]?.content;

  if (content === undefined) return {};

  return minimalOf(schemas, resolveSchema(schemas, content["application/json"].schema.$ref));
}

/** True when a path template (`/api/x/{id}`) matches a concrete request path, `{...}` segments aside. */
function pathMatches(template: string, pathname: string): boolean {
  const templateSegments = template.split("/");
  const pathSegments = pathname.split("/");

  if (templateSegments.length !== pathSegments.length) return false;

  return templateSegments.every(
    (segment, index) =>
      (segment.startsWith("{") && segment.endsWith("}")) || segment === pathSegments[index],
  );
}

/** How many `{...}` segments a path template carries - fewer is a more specific match. */
function paramCount(template: string): number {
  return template.split("/").filter((segment) => segment.startsWith("{") && segment.endsWith("}"))
    .length;
}

/**
 * The GET operation whose path template matches pathname, preferring the
 * most specific (fewest-param) template - a static path like
 * `/api/x/items/search` and a member path like `/api/x/items/{id}` can
 * both match the same segment count, and only one of them is real.
 */
function findGetOperation(
  paths: Readonly<Record<string, PathItem>>,
  pathname: string,
): Operation | undefined {
  const candidates = Object.entries(paths).filter(
    ([template, item]) => item.get !== undefined && pathMatches(template, pathname),
  );

  candidates.sort(([a], [b]) => paramCount(a) - paramCount(b));

  return candidates[0]?.[1].get;
}

/** Starts the fixture server on port, resolving once it is listening. */
export function start(port: number): Promise<Serving> {
  const byName = new Map(services().map((service) => [service.name, service]));

  const server = createServer((req, res) => {
    const pathname = (req.url ?? "").split("?")[0] ?? "";
    const specMatch = /^\/([^/]+)\/openapi\.yaml$/u.exec(pathname);

    if (specMatch !== null) {
      const service = byName.get(specMatch[1] ?? "");

      if (service !== undefined) {
        res.writeHead(200, { "content-type": "application/yaml" });
        res.end(JSON.stringify(toOpenAPI(service)));

        return;
      }
    } else if (req.method === "GET") {
      const invokeMatch = /^\/api\/([^/]+)\//u.exec(pathname);
      const service = invokeMatch === null ? undefined : byName.get(invokeMatch[1] ?? "");
      const doc = service === undefined ? undefined : toOpenAPI(service);
      const operation = doc === undefined ? undefined : findGetOperation(doc.paths, pathname);

      if (doc !== undefined && operation !== undefined) {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(JSON.stringify(fixtureBody(doc.components.schemas, operation)));

        return;
      }
    }

    res.writeHead(404, { "content-type": "text/plain" });
    res.end("not found");
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
