/**
 * The narrowing fixture's definition table types.
 *
 * A person reads and edits values of these types; `openapi.ts` synthesises
 * OpenAPI documents from them. See docs/specs/narrowing.md section 3 and 4.
 */

export type Verb = "list" | "get" | "create" | "update" | "delete";

/** One resource a service owns: five operations at most, one noun. */
export interface Resource {
  /** Singular, PascalCase, unique inside a service: "SalesOrder". */
  readonly id: string;
  /** Plural form used by list: "SalesOrders". */
  readonly plural: string;
  /** The Japanese noun every summary for this resource is built from. */
  readonly noun: string;
  readonly verbs: readonly Verb[];
  /**
   * Axis C. Resources sharing a group key are near-neighbours inside one
   * service: 在庫品目 / 在庫ロット / 在庫引当 / 棚卸 / 在庫調整.
   */
  readonly group?: string;
  /**
   * Axis B. Resources sharing a key across services carry the same Japanese
   * noun in two or more places: "order" is 受注 in sales and 発注 in
   * purchasing, and both summarise as 注文.
   */
  readonly shared?: string;
  /** Extra vocabulary the description carries, for realism. */
  readonly also?: readonly string[];
  /**
   * `x-orchestra-examples` for every operation this resource generates:
   * things a person might type when they want it (docs/specs/describing.md,
   * section 3). Optional; written blind, by Task 4, never here (G6).
   */
  readonly examples?: readonly string[];
}

/** Axis E lives here: settings named after the transactions they configure. */
export interface Setting {
  readonly id: string;
  readonly verb: "get" | "update" | "list";
  /** Written by hand, because the decoy is the whole point. */
  readonly summary: string;
  readonly displayName: string;
  /** `x-orchestra-examples` for the operation this setting generates. */
  readonly examples?: readonly string[];
}

export interface Aggregate {
  readonly id: string;
  readonly kind: "search" | "summarize" | "aggregate";
  readonly noun: string;
  /** `x-orchestra-examples` for the operation this aggregate generates. */
  readonly examples?: readonly string[];
}

export interface Workflow {
  readonly id: string;
  readonly noun: string;
  readonly actions: readonly ("submit" | "approve" | "reject" | "withdraw")[];
  /**
   * `x-orchestra-examples` for every operation this workflow generates -
   * one per action, all sharing this list, exactly as Resource's does.
   */
  readonly examples?: readonly string[];
}

export interface ServiceFixture {
  readonly name: string;
  readonly displayName: string;
  readonly resources: readonly Resource[];
  readonly aggregates: readonly Aggregate[];
  readonly settings: readonly Setting[];
  readonly workflows: readonly Workflow[];
}
