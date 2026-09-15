import type { PlanResult } from "@/shared/api/client";

/** One shortlist candidate a `result` carries, already narrowed to the three fields this feature reads. */
export interface Alternative {
  readonly operationId: string;
  readonly displayName: string;
  readonly service: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * `PlanResult.alternatives`, read defensively rather than iterated
 * directly. openapi-fetch's `MethodResponse` mapping turns a nested
 * array-of-objects schema property into a type whose own prototype methods
 * (`map`, `filter`, ...) type-check as `{}` - calling `.map` on it directly
 * is a compile error, not just an unsafe read (`ResultChoice`'s own
 * `toEnumOptions` carries the same comment for `PlanResult.options`).
 * `Array.isArray` narrows to a real `any[]`, which is what restores
 * working array methods; the shape check afterwards drops anything that
 * does not carry all three fields as strings.
 *
 * Shared by `conversationStore` - which needs to know, before it appends
 * any turn, whether a `result` carries anything worth its own "違いましたか？"
 * turn - and `AlternativesRow`, which draws the chips that turn holds. Both
 * would otherwise carry this same parsing.
 */
export function toAlternatives(alternatives: PlanResult["alternatives"]): readonly Alternative[] {
  if (!Array.isArray(alternatives)) {
    return [];
  }

  return alternatives.flatMap((alternative: unknown) => {
    if (!isRecord(alternative)) {
      return [];
    }

    const { operationId, displayName, service } = alternative;

    return typeof operationId === "string" &&
      typeof displayName === "string" &&
      typeof service === "string"
      ? [{ operationId, displayName, service }]
      : [];
  });
}
