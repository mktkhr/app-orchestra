/**
 * The mid subset (docs/specs/midsizing.md, M1-M2): three of the fixture's
 * five services, two resources each, with one id `pattern` prefix per
 * service. Built entirely from the full services' own definition tables -
 * every kept operation's written examples and generated body are exactly
 * what the full fixture already produces, so a subset costs a list of
 * thirty operation ids and an id prefix per service, plus (below,
 * `midOperationInfo`) reading each operation's own request body schema for
 * `score-mid.ts`'s fabrication check - added in the Task 3 review, after
 * the first `make eval-mid` run counted every schema-declared `status`
 * enum value (`openapi-document.ts`'s shared `FixtureStatus`) as an
 * invented string, which the plan's own rule ("not enum") excludes.
 */
import { isRecord } from "../../src/helpers/wire.ts";
import { attendance } from "./attendance.ts";
import type { FixtureOperation } from "./operations.ts";
import { operationsOf } from "./operations.ts";
import { toOpenAPI } from "./openapi.ts";
import type { Operation, SchemaObject } from "./openapi.ts";
import { purchasing } from "./purchasing.ts";
import { sales } from "./sales.ts";
import type { ServiceFixture } from "./types.ts";

export const MID_RESOURCES = {
  sales: ["Order", "Partner"],
  purchasing: ["Order", "Partner"],
  attendance: ["Employee", "LeaveRequest"],
} as const;

export const MID_ID_PREFIX = { sales: "so-", purchasing: "po-", attendance: "att-" } as const;

type MidServiceName = keyof typeof MID_RESOURCES;

const MID_SERVICE_NAMES: readonly MidServiceName[] = ["sales", "purchasing", "attendance"];

const FULL: Readonly<Record<MidServiceName, ServiceFixture>> = { sales, purchasing, attendance };

/** One full service cut to its two `MID_RESOURCES`, aggregates/settings/workflows dropped, idPrefix set. */
function midServiceOf(name: MidServiceName): ServiceFixture {
  const full = FULL[name];
  const keep: readonly string[] = MID_RESOURCES[name];

  return {
    name: full.name,
    displayName: full.displayName,
    resources: full.resources.filter((resource) => keep.includes(resource.id)),
    aggregates: [],
    settings: [],
    workflows: [],
    idPrefix: MID_ID_PREFIX[name],
  };
}

/** The three services cut to MID_RESOURCES, aggregates/settings/workflows dropped, idPrefix set. */
export function midServices(): readonly ServiceFixture[] {
  return MID_SERVICE_NAMES.map((name) => midServiceOf(name));
}

/** The thirty operations, in the same order operationsOf yields them. */
export function midCatalog(): readonly FixtureOperation[] {
  return midServices().flatMap((service) => operationsOf(service));
}

type Schemas = Readonly<Record<string, SchemaObject>>;

/** The schema definitions object a `$ref` like `#/components/schemas/Foo` points into - the same lookup serve.ts's own `resolveSchema` does, duplicated rather than shared across the two files' different concerns (serving a body vs. reading a schema's own enum values). */
function resolveSchema(schemas: Schemas, ref: string): SchemaObject {
  const name = ref.slice(ref.lastIndexOf("/") + 1);
  const schema = schemas[name];

  if (schema === undefined) throw new Error(`mid enum values: unknown schema ${ref}`);

  return schema;
}

function isSchemaRef(value: unknown): value is { readonly $ref: string } {
  return isRecord(value) && typeof value["$ref"] === "string";
}

/**
 * Every enum value declared on operation's own request body schema, one
 * `$ref` deep - every property of every fixture schema is either a plain
 * type or a single `$ref` (`openapi-document.ts`'s own `SCHEMAS`), never a
 * nested object, so one level is everything there is to walk. Empty for a
 * read-only operation (`list`/`get`, no request body) or one whose request
 * body has no enum property.
 */
function requestBodyEnumValues(schemas: Schemas, operation: Operation): ReadonlySet<string> {
  const values = new Set<string>();

  if (operation.requestBody === undefined) return values;

  const schema = resolveSchema(
    schemas,
    operation.requestBody.content["application/json"].schema.$ref,
  );

  if (!("properties" in schema)) return values;

  for (const property of Object.values(schema.properties)) {
    if (!isSchemaRef(property)) continue;

    const resolved = resolveSchema(schemas, property.$ref);

    if ("enum" in resolved) for (const value of resolved.enum) values.add(value);
  }

  return values;
}

/** One mid operation's enum values, as `score-mid.ts`'s fabrication check needs them - a string `initial` value that is one of these is never counted as invented, per docs/specs/midsizing.md M4's "not enum". */
export interface MidOperationInfo {
  readonly enumValues: ReadonlySet<string>;
}

/** `midCatalog()`'s own operation ids, each mapped to its request body's enum values (docs/plans/midsizing.md Task 3 review). */
export function midOperationInfo(): ReadonlyMap<string, MidOperationInfo> {
  const info = new Map<string, MidOperationInfo>();

  for (const service of midServices()) {
    const doc = toOpenAPI(service);

    for (const item of Object.values(doc.paths)) {
      for (const operation of [item.get, item.post, item.put, item.delete]) {
        if (operation === undefined) continue;

        info.set(operation.operationId, {
          enumValues: requestBodyEnumValues(doc.components.schemas, operation),
        });
      }
    }
  }

  return info;
}
