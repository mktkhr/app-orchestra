/**
 * The minimal OpenAPI 3.0.3 document shape `openapi.ts` synthesises, plus
 * the schemas every fixture operation shares. Split out of `openapi.ts` to
 * stay under this repository's per-file line policy.
 */

export interface Response {
  readonly description: string;
  readonly content?: { readonly "application/json": { readonly schema: SchemaRef } };
}

export interface RequestBody {
  readonly required: true;
  readonly content: { readonly "application/json": { readonly schema: SchemaRef } };
}

/** A path parameter carrying a `pattern` constraint (the mid subset's id affinity, M2). */
export interface Parameter {
  readonly name: string;
  readonly in: "path";
  readonly required: true;
  readonly schema: { readonly type: "string"; readonly pattern: string };
}

export interface Operation {
  readonly operationId: string;
  readonly summary: string;
  readonly description: string;
  readonly tags: readonly string[];
  readonly "x-orchestra-expose": true;
  readonly "x-ui-hint": { readonly displayName: string };
  readonly requestBody?: RequestBody;
  readonly parameters?: readonly Parameter[];
  readonly responses: Readonly<Record<string, Response>>;
  /**
   * Things a person might type when they want this operation
   * (docs/specs/describing.md, section 3). Absent when the definition
   * table's entry carries none - `toOpenAPI` emits it only when present
   * (`exactOptionalPropertyTypes`).
   */
  readonly "x-orchestra-examples"?: readonly string[];
}

export interface PathItem {
  readonly get?: Operation;
  readonly post?: Operation;
  readonly put?: Operation;
  readonly delete?: Operation;
}

interface SchemaRef {
  readonly $ref: string;
}

interface EnumSchema {
  readonly type: "string";
  readonly title: string;
  readonly description: string;
  readonly enum: readonly string[];
  readonly "x-enum-labels": Readonly<Record<string, string>>;
}

interface ObjectSchema {
  readonly type: "object";
  readonly description: string;
  readonly required: readonly string[];
  readonly properties: Readonly<Record<string, unknown>>;
}

export type SchemaObject = EnumSchema | ObjectSchema;

export interface OpenAPIDocument {
  readonly openapi: "3.0.3";
  readonly info: {
    readonly title: string;
    readonly version: string;
    readonly description: string;
    readonly "x-ui-hint": { readonly displayName: string };
  };
  readonly servers: readonly { readonly url: string; readonly description: string }[];
  readonly security: readonly unknown[];
  readonly tags: readonly { readonly name: string; readonly description: string }[];
  readonly paths: Readonly<Record<string, PathItem>>;
  readonly components: { readonly schemas: Readonly<Record<string, SchemaObject>> };
}

/** One status enum, reused everywhere, so exactly one x-enum-labels exists per document. */
export const SCHEMAS: Readonly<Record<string, SchemaObject>> = {
  FixtureStatus: {
    type: "string",
    title: "ステータス",
    description: "A fixture record's lifecycle state.",
    enum: ["active", "inactive", "pending", "archived"],
    "x-enum-labels": {
      active: "有効",
      inactive: "無効",
      pending: "保留",
      archived: "アーカイブ済み",
    },
  },
  FixtureRecord: {
    type: "object",
    description: "A single fixture record. The fixture serves contracts, not data.",
    required: ["id", "name", "status"],
    properties: {
      id: { type: "string", title: "ID", description: "The record's id." },
      name: { type: "string", title: "名称", description: "The record's name." },
      status: { $ref: "#/components/schemas/FixtureStatus" },
    },
  },
  FixtureRecordList: {
    type: "object",
    description: "A page of fixture records.",
    required: ["items"],
    properties: {
      items: {
        type: "array",
        description: "The matching records.",
        items: { $ref: "#/components/schemas/FixtureRecord" },
      },
    },
  },
  NewFixtureRecord: {
    type: "object",
    description: "The fields needed to create a fixture record.",
    required: ["name", "status"],
    properties: {
      name: { type: "string", title: "名称", description: "The record's name." },
      status: { $ref: "#/components/schemas/FixtureStatus" },
    },
  },
  Error: {
    type: "object",
    description: "A machine-readable error.",
    required: ["message"],
    properties: { message: { type: "string", description: "What went wrong, for humans." } },
  },
};
