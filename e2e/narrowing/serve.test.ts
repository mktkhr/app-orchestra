import { afterAll, beforeAll, describe, expect, test } from "vite-plus/test";

import { operationsOf, services } from "./fixture/index.ts";
import { toOpenAPI } from "./fixture/openapi.ts";
import type { OpenAPIDocument, Operation, SchemaObject } from "./fixture/openapi.ts";
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

/** `/api/x/{id}/y` with every `{...}` segment filled in, for a concrete request. */
function concretePath(template: string): string {
  return template.replaceAll(/\{[^}]+\}/gu, "1");
}

/** Every GET operation a document declares, paired with a concrete request path for it. */
function getInvokes(
  doc: OpenAPIDocument,
): readonly { readonly path: string; readonly operation: Operation }[] {
  return Object.entries(doc.paths).flatMap(([template, item]) =>
    item.get === undefined ? [] : [{ path: concretePath(template), operation: item.get }],
  );
}

/** The schema a `$ref` like `#/components/schemas/Foo` names, from a document's own components. */
function schemaNamed(doc: OpenAPIDocument, ref: string): SchemaObject {
  const name = ref.slice(ref.lastIndexOf("/") + 1);
  const schema = doc.components.schemas[name];

  if (schema === undefined) throw new Error(`test fixture: unknown schema ${ref}`);

  return schema;
}

/** Whether value validates against schema - re-derived here from toOpenAPI's own output, not hand-copied field names. */
function validatesAgainst(doc: OpenAPIDocument, schema: SchemaObject, value: unknown): boolean {
  if ("enum" in schema) return typeof value === "string" && schema.enum.includes(value);

  return (
    isRecord(value) && schema.required.every((key) => hasValidProperty(doc, schema, key, value))
  );
}

function hasValidProperty(
  doc: OpenAPIDocument,
  schema: Extract<SchemaObject, { readonly type: "object" }>,
  key: string,
  value: Readonly<Record<string, unknown>>,
): boolean {
  const property = schema.properties[key];
  const propertyValue = value[key];

  if (!isRecord(property)) return false;

  const ref = property["$ref"];

  if (typeof ref === "string") return validatesAgainst(doc, schemaNamed(doc, ref), propertyValue);
  if (property["type"] === "array") return Array.isArray(propertyValue);

  return true;
}

/** The schema an operation's 200 response declares, or undefined when it declares no body. */
function responseSchema(doc: OpenAPIDocument, operation: Operation): SchemaObject | undefined {
  const content = operation.responses["200"]?.content;

  return content === undefined
    ? undefined
    : schemaNamed(doc, content["application/json"].schema.$ref);
}

/** One GET invoke's outcome: the status it answered with, and whether its body validates. */
interface GetOutcome {
  readonly status: number;
  readonly valid: boolean;
}

/** Fetches every GET invoke path a document declares and checks each body against its own schema. */
function fetchGetOutcomes(
  origin: string,
  serviceName: string,
  doc: OpenAPIDocument,
): Promise<readonly GetOutcome[]> {
  return Promise.all(
    getInvokes(doc).map(async ({ path, operation }) => {
      // The shape the platform really sends: base URL `/<service>` + path.
      const response = await fetch(`${origin}/${serviceName}${path}`);
      const body: unknown = await response.json();
      const schema = responseSchema(doc, operation);
      const valid = schema === undefined || validatesAgainst(doc, schema, body);

      return { status: response.status, valid };
    }),
  );
}

/** Every outcome answered 200 with a body that validated. */
function allValid(outcomes: readonly GetOutcome[]): boolean {
  return outcomes.every((o) => o.status === 200 && o.valid);
}

/** Every concrete path a document exposes for creation (POST) - the unsafe half this fixture never answers. */
function postPathsOf(doc: OpenAPIDocument): readonly string[] {
  return Object.entries(doc.paths).flatMap(([template, item]) =>
    item.post === undefined ? [] : [concretePath(template)],
  );
}

/** POSTs to every one of a document's create paths and reports each response's status. */
function fetchPostStatuses(origin: string, doc: OpenAPIDocument): Promise<readonly number[]> {
  return Promise.all(
    postPathsOf(doc).map(
      async (path) => (await fetch(`${origin}${path}`, { method: "POST" })).status,
    ),
  );
}

/** Every one of these statuses was 404. */
function allNotFound(statuses: readonly number[]): boolean {
  return statuses.every((status) => status === 404);
}

/** The first fixture service's name, for a test that only needs any one known service. */
function firstServiceName(): string {
  return services()[0]?.name ?? "";
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

  test("every GET invoke path answers 200 with a body matching its own declared response schema", async () => {
    const doc = toOpenAPI(service);
    const outcomes = await fetchGetOutcomes(baseUrl, service.name, doc);

    expect(getInvokes(doc).length).toBeGreaterThan(0);
    expect(allValid(outcomes)).toBe(true);
  });

  test("a POST invoke path still 404s - unsafe operations never reach the service", async () => {
    const doc = toOpenAPI(service);
    const statuses = await fetchPostStatuses(baseUrl, doc);

    expect(postPathsOf(doc).length).toBeGreaterThan(0);
    expect(allNotFound(statuses)).toBe(true);
  });
});

test("404s for an unknown service", async () => {
  const response = await fetch(`${baseUrl}/not-a-service/openapi.yaml`);

  expect(response.status).toBe(404);
});

test("404s a GET under a known service's /api path that matches no declared operation", async () => {
  const response = await fetch(`${baseUrl}/api/${firstServiceName()}/not-a-real-endpoint`);

  expect(response.status).toBe(404);
});
