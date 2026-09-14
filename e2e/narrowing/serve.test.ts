import { afterAll, beforeAll, describe, expect, test } from "vite-plus/test";

import { operationsOf, services } from "./fixture/index.ts";
import { start, type Serving } from "./serve.ts";

/**
 * The fixture serves contracts only, over `node:http`, on a port this test
 * picks itself (port 0) so it needs nothing already running - see
 * docs/plans/narrowing.md Task 2.
 */

let serving: Serving;
let baseUrl: string;

/** True when value is a JSON object - not an array, not null. */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * Every `operationId` string found anywhere in a parsed JSON document, by
 * runtime check rather than typing the response body - the served document's
 * shape is Task 1's concern, not this test's. `JSON.parse` is enough because
 * every JSON document is already a valid YAML document: it reads the served
 * body back exactly the way the platform's YAML-tolerant loader does.
 */
function collectOperationIds(value: unknown, found: Set<string>): Set<string> {
  if (Array.isArray(value)) {
    for (const item of value) collectOperationIds(item, found);

    return found;
  }

  if (!isRecord(value)) return found;

  const { operationId } = value;

  if (typeof operationId === "string") found.add(operationId);

  for (const child of Object.values(value)) collectOperationIds(child, found);

  return found;
}

async function fetchOperationIds(name: string): Promise<Set<string>> {
  const response = await fetch(`${baseUrl}/${name}/openapi.yaml`);
  const body: unknown = JSON.parse(await response.text());

  return collectOperationIds(body, new Set());
}

beforeAll(async () => {
  serving = await start(0);
  const address = serving.server.address();

  if (!isRecord(address)) throw new Error("the fixture server did not report a listening address");

  baseUrl = `http://127.0.0.1:${String(address["port"])}`;
});

afterAll(async () => {
  await serving.stop();
});

describe.each(services())("$name", (service) => {
  test("responds 200 with an application/yaml body", async () => {
    const response = await fetch(`${baseUrl}/${service.name}/openapi.yaml`);

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toContain("application/yaml");
  });

  test("parses back to the same operation ids the fixture defines", async () => {
    const ids = await fetchOperationIds(service.name);
    const expected = new Set(operationsOf(service).map((op) => op.operationId));

    expect(ids).toStrictEqual(expected);
  });
});

test("404s for an unknown service", async () => {
  const response = await fetch(`${baseUrl}/not-a-service/openapi.yaml`);

  expect(response.status).toBe(404);
});
