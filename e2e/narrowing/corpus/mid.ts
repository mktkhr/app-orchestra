/**
 * The mid corpus: sixty questions over the thirty-operation mid subset
 * (docs/plans/midsizing.md Task 2, docs/specs/midsizing.md section 4).
 * The questions themselves live in `mid-answerable.ts`, `mid-collision.ts`
 * and `mid-impossible.ts` (split to stay under 300 lines); this file only
 * combines them, the way `index.ts` combines the shortlist corpus's axes.
 *
 * Written blind: the author read only the resource nouns and the five verbs
 * — 受注 / 取引先 (sales), 発注 / 取引先 (purchasing), 社員 / 休暇申請
 * (attendance), each with list/get/create/update/delete — and the three id
 * prefixes (`so-`, `po-`, `att-`). Nothing under `fixture/examples-*.ts` or
 * the resource summaries in `fixture/sales.ts` / `purchasing.ts` /
 * `attendance.ts` was opened while writing these.
 *
 * `axis` is reused from the shortlist corpus's `Question` but is
 * informational for `mid`: the subset has no settings/aggregate operation
 * to build an axis E decoy from and no same-service near-neighbour group
 * for axis C, so only A (a single defensible answer), B (the subset's
 * sales/purchasing collision on 注文 and 取引先) and D (a paraphrase using
 * 得意先/仕入先 instead of 取引先) occur among the answerable half; every
 * impossible question is axis A, per docs/plans/midsizing.md Task 2.
 *
 * Answerable (40, m01-m40): one question per operation (m01-m30, in
 * catalogue order), plus ten collision questions (m31-m40) naming both
 * sales and purchasing for every verb on 注文 and on 取引先. Twenty-two of
 * the forty carry an argument: an id in the service's prefix, a name on a
 * create, or a duration on the one leave-request create. Neither of the six
 * resources declares an enum in the fixture's `Resource` type
 * (`fixture/types.ts` has no such field), so no question uses one — ids and
 * names only, as the plan allows.
 *
 * Impossible (20, m41-m60): five verb-not-there, five resource-not-there
 * (a resource that exists in the full fixture but was cut from the mid
 * subset, or belongs to a service outside it), five out of domain, five
 * capability (two general, three scoped — `capability: true`).
 */
import { MID_ANSWERABLE } from "./mid-answerable.ts";
import { MID_COLLISION } from "./mid-collision.ts";
import { MID_IMPOSSIBLE } from "./mid-impossible.ts";
import type { MidQuestion } from "./types.ts";

export const MID_QUESTIONS: readonly MidQuestion[] = [
  ...MID_ANSWERABLE,
  ...MID_COLLISION,
  ...MID_IMPOSSIBLE,
];

export function midQuestions(): readonly MidQuestion[] {
  return MID_QUESTIONS;
}
