/**
 * The mid subset (docs/specs/midsizing.md, M1-M2): three of the fixture's
 * five services, two resources each, with one id `pattern` prefix per
 * service. Built entirely from the full services' own definition tables -
 * every kept operation's written examples and generated body are exactly
 * what the full fixture already produces, so a subset costs a list of
 * thirty operation ids and an id prefix per service, nothing else.
 */
import { attendance } from "./attendance.ts";
import type { FixtureOperation } from "./operations.ts";
import { operationsOf } from "./operations.ts";
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
