/**
 * Synthesises one OpenAPI 3.0.3 document from a `ServiceFixture` (spec T5:
 * the repository holds the definition table, not the generated contracts).
 *
 * Every exposed operation carries `x-orchestra-expose: true` and
 * `x-ui-hint.displayName`, matching the rules
 * `harness/guard/exposed-ops.sh` and the Redocly ruleset apply to
 * `services/*\/api/openapi.yaml` — this fixture is not one of those services
 * (spec T4), so the tests in `openapi.test.ts` assert the same rules by hand.
 */
import { capitalize, kebabOf } from "./naming.ts";
import { SCHEMAS } from "./openapi-document.ts";
import type {
  OpenAPIDocument,
  Operation,
  PathItem,
  RequestBody,
  Response,
} from "./openapi-document.ts";
import type { Aggregate, Resource, ServiceFixture, Setting, Verb, Workflow } from "./types.ts";

export type {
  OpenAPIDocument,
  Operation,
  PathItem,
  RequestBody,
  Response,
  SchemaObject,
} from "./openapi-document.ts";

const SHARED_DISPLAY: Readonly<Record<string, string>> = {
  order: "注文",
  line: "明細",
  approval: "承認",
  employee: "社員",
  partner: "取引先",
};

function response(description: string, schema?: string): Response {
  if (schema === undefined) return { description };

  return {
    description,
    content: { "application/json": { schema: { $ref: `#/components/schemas/${schema}` } } },
  };
}

function requestBodyOf(schema: string): RequestBody {
  return {
    required: true,
    content: { "application/json": { schema: { $ref: `#/components/schemas/${schema}` } } },
  };
}

const CRUD_SUMMARY: Readonly<Record<Verb, (noun: string) => string>> = {
  list: (noun) => `${noun}の一覧`,
  get: (noun) => `${noun}を1件取得`,
  create: (noun) => `${noun}の作成`,
  update: (noun) => `${noun}の更新`,
  delete: (noun) => `${noun}の削除`,
};

function crudSummary(verb: Verb, noun: string): string {
  return CRUD_SUMMARY[verb](noun);
}

const CRUD_DISPLAY_NAME: Readonly<Record<Verb, (base: string) => string>> = {
  list: (base) => `${base}一覧`,
  get: (base) => `${base}詳細`,
  create: (base) => `${base}の作成`,
  update: (base) => `${base}の更新`,
  delete: (base) => `${base}の削除`,
};

function crudDisplayName(verb: Verb, base: string): string {
  return CRUD_DISPLAY_NAME[verb](base);
}

function baseDisplayOf(resource: Resource): string {
  if (resource.shared === undefined) return resource.noun;

  return SHARED_DISPLAY[resource.shared] ?? resource.noun;
}

const VERB_LABEL: Readonly<Record<Verb, string>> = {
  list: "一覧取得",
  get: "取得",
  create: "作成",
  update: "更新",
  delete: "削除",
};

function descriptionOf(resource: Resource, verb: Verb): string {
  const also =
    resource.also === undefined || resource.also.length === 0
      ? ""
      : `関連語: ${resource.also.join("、")}。`;

  return `${resource.noun}に対する${VERB_LABEL[verb]}操作。${also}`;
}

const CRUD_RESPONSES: Readonly<Record<Verb, Readonly<Record<string, Response>>>> = {
  list: { "200": response("The matching records.", "FixtureRecordList") },
  create: { "201": response("The created record.", "FixtureRecord") },
  get: {
    "200": response("The requested record.", "FixtureRecord"),
    "404": response("No record with this id.", "Error"),
  },
  update: {
    "200": response("The updated record.", "FixtureRecord"),
    "404": response("No record with this id.", "Error"),
  },
  delete: {
    "200": response("The record after deletion.", "FixtureRecord"),
    "404": response("No record with this id.", "Error"),
  },
};

function crudOperation(service: string, resource: Resource, verb: Verb): Operation {
  const idPart = verb === "list" ? resource.plural : resource.id;

  return {
    operationId: `${verb}${capitalize(service)}${idPart}`,
    summary: crudSummary(verb, resource.noun),
    description: descriptionOf(resource, verb),
    tags: [service],
    "x-orchestra-expose": true,
    "x-ui-hint": { displayName: crudDisplayName(verb, baseDisplayOf(resource)) },
    ...(verb === "create" || verb === "update"
      ? { requestBody: requestBodyOf("NewFixtureRecord") }
      : {}),
    responses: CRUD_RESPONSES[verb],
  };
}

/** A resource's two paths: the collection, and one member by id. */
function resourcePaths(service: string, resource: Resource): Readonly<Record<string, PathItem>> {
  const base = `/api/${service}/${kebabOf(resource.plural)}`;
  const byId = `${base}/{id}`;
  const has = (v: Verb): boolean => resource.verbs.includes(v);

  return {
    [base]: {
      ...(has("list") ? { get: crudOperation(service, resource, "list") } : {}),
      ...(has("create") ? { post: crudOperation(service, resource, "create") } : {}),
    },
    [byId]: {
      ...(has("get") ? { get: crudOperation(service, resource, "get") } : {}),
      ...(has("update") ? { put: crudOperation(service, resource, "update") } : {}),
      ...(has("delete") ? { delete: crudOperation(service, resource, "delete") } : {}),
    },
  };
}

const AGGREGATE_LABEL: Readonly<Record<Aggregate["kind"], string>> = {
  search: "検索",
  summarize: "集計",
  aggregate: "集計値算出",
};

function aggregatePath(service: string, aggregate: Aggregate): Readonly<Record<string, PathItem>> {
  const label = AGGREGATE_LABEL[aggregate.kind];
  const path = `/api/${service}/${kebabOf(aggregate.id)}/${aggregate.kind}`;
  const operation: Operation = {
    operationId: `${aggregate.kind}${capitalize(service)}${aggregate.id}`,
    summary: `${aggregate.noun}を${label}する`,
    description: `${aggregate.noun}に対する${label}を行い、結果をまとめて返す。`,
    tags: [service],
    "x-orchestra-expose": true,
    "x-ui-hint": { displayName: `${aggregate.noun}${label}` },
    responses: { "200": response(`${aggregate.noun}の${label}結果。`, "FixtureRecordList") },
  };

  return { [path]: { get: operation } };
}

function settingPath(service: string, setting: Setting): Readonly<Record<string, PathItem>> {
  const path = `/api/${service}/settings/${kebabOf(setting.id)}`;
  const operation: Operation = {
    operationId: `${setting.verb}${capitalize(service)}${setting.id}`,
    summary: setting.summary,
    description: `${setting.summary}。設定・マスタ系のAPIで、対象の取引そのものは扱わない。`,
    tags: [service, "settings"],
    "x-orchestra-expose": true,
    "x-ui-hint": { displayName: setting.displayName },
    ...(setting.verb === "update" ? { requestBody: requestBodyOf("NewFixtureRecord") } : {}),
    responses: {
      "200": response(
        "The setting value.",
        setting.verb === "list" ? "FixtureRecordList" : "FixtureRecord",
      ),
    },
  };

  return { [path]: { [setting.verb === "update" ? "put" : "get"]: operation } };
}

const ACTION_LABEL: Readonly<Record<Workflow["actions"][number], string>> = {
  submit: "申請",
  approve: "承認",
  reject: "却下",
  withdraw: "取下げ",
};

function workflowPaths(service: string, workflow: Workflow): Readonly<Record<string, PathItem>> {
  const paths: Record<string, PathItem> = {};

  for (const action of workflow.actions) {
    const label = ACTION_LABEL[action];
    const path = `/api/${service}/${kebabOf(workflow.id)}/{id}/${action}`;
    const operation: Operation = {
      operationId: `${action}${capitalize(service)}${workflow.id}`,
      summary: `${workflow.noun}を${label}する`,
      description: `${workflow.noun}のワークフローで${label}操作を行う。`,
      tags: [service, "workflow"],
      "x-orchestra-expose": true,
      "x-ui-hint": { displayName: `${workflow.noun}${label}` },
      responses: { "200": response(`${label}後の状態。`, "FixtureRecord") },
    };

    paths[path] = { post: operation };
  }

  return paths;
}

/** Builds one OpenAPI 3.0.3 document from a service's definition table. */
export function toOpenAPI(fixture: ServiceFixture): OpenAPIDocument {
  const paths: Record<string, PathItem> = {};
  const merge = (more: Readonly<Record<string, PathItem>>): void => {
    Object.assign(paths, more);
  };

  for (const resource of fixture.resources) merge(resourcePaths(fixture.name, resource));
  for (const aggregate of fixture.aggregates) merge(aggregatePath(fixture.name, aggregate));
  for (const setting of fixture.settings) merge(settingPath(fixture.name, setting));
  for (const workflow of fixture.workflows) merge(workflowPaths(fixture.name, workflow));

  return {
    openapi: "3.0.3",
    info: {
      title: `${fixture.displayName} API`,
      version: "0.1.0",
      description: `Narrowing fixture service: ${fixture.displayName}. See docs/specs/narrowing.md.`,
      "x-ui-hint": { displayName: fixture.displayName },
    },
    servers: [{ url: "/", description: "The narrowing fixture serves its own contract." }],
    security: [],
    tags: [{ name: fixture.name, description: `${fixture.displayName}が扱う操作。` }],
    paths,
    components: { schemas: SCHEMAS },
  };
}
