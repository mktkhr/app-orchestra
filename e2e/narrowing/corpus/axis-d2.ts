/**
 * Axis D extension (docs/specs/narrowing.md section 4): 25 more vocabulary-gap
 * questions, same kind as `axis-d.ts` — each shares no character bigram at
 * all with its answer's combined text (summary, description, display name,
 * service display name), asserted in `corpus-extension.test.ts` via
 * `bigramsOf`, not just claimed.
 *
 * Held in a separate file, and exposed only through `extensionQuestions()`
 * (`index.ts`), so the original 100 questions in `questions()` — and the
 * numbers already recorded against them — do not move.
 *
 * The same systemic trap `axis-d.ts` names applies here: every CRUD
 * operation's `description` is built as `${noun}に対する${label}操作。`,
 * which always contains the bigram "する" (from 対する), and every settings
 * operation's `description` ends in `...を扱わない。`, which always contains
 * the bigram "ない". A question phrased with a `〜する` verb ending, or one
 * that uses a plain negation, will often collide with one of those by
 * accident; every question here ends in `〜したい` instead, and none of them
 * has an answer whose combined text happens to end in a settings
 * description, which is what makes `〜ない` safe here.
 */
import type { Question } from "./types.ts";

export const AXIS_D2: readonly Question[] = [
  {
    id: "d2-01",
    axis: "D",
    text: "誰が来ているか確かめたい",
    answers: ["listAttendanceClockEvents"],
  },
  {
    id: "d2-02",
    axis: "D",
    text: "会社を休むと伝えたい",
    answers: ["createAttendanceAbsence"],
  },
  {
    id: "d2-03",
    axis: "D",
    text: "働く曜日を変えてほしいと頼みたい",
    answers: ["updateAttendanceWorkSchedule"],
  },
  {
    id: "d2-04",
    axis: "D",
    text: "今どこで働いているか登録したい",
    answers: ["createAttendanceWorkLocation"],
  },
  {
    id: "d2-05",
    axis: "D",
    text: "体の具合を診てもらった証拠を残したい",
    answers: ["createAttendanceHealthCheckup"],
  },
  {
    id: "d2-06",
    axis: "D",
    // "頼みたい" admits placing the order outright as well as raising the
    // internal purchase request that precedes it; the question does not
    // decide which stage the person means.
    text: "足りなくなった部品を仕入れてほしいと頼みたい",
    answers: ["createPurchasingOrder", "createPurchasingPurchaseRequest"],
  },
  {
    id: "d2-07",
    axis: "D",
    text: "急いで足りない分を買いたい",
    answers: ["createPurchasingEmergencyOrder"],
  },
  {
    id: "d2-08",
    axis: "D",
    text: "壊れていた荷物を送り返したい",
    answers: ["createPurchasingSupplierReturn"],
  },
  {
    id: "d2-09",
    axis: "D",
    text: "取引先の仕事ぶりを点数にしたい",
    answers: ["createPurchasingSupplierEvaluation"],
  },
  {
    id: "d2-10",
    axis: "D",
    text: "海外からの荷物を通す手続きをしたい",
    answers: ["createPurchasingImportDeclaration"],
  },
  {
    id: "d2-11",
    axis: "D",
    text: "現品を数えて記録に残したい",
    answers: ["createInventoryStockCount"],
  },
  {
    id: "d2-12",
    axis: "D",
    text: "荷物をまとめて積み上げたい",
    answers: ["createInventoryPallet"],
  },
  {
    id: "d2-13",
    axis: "D",
    text: "新しい箱に詰め替えたい",
    answers: ["createInventoryPackingList"],
  },
  {
    id: "d2-14",
    axis: "D",
    // "出庫前に集める順番" names what a pick list is for (the order items are
    // gathered in before shipping), not the packing list itself.
    text: "出庫前に集める順番のメモを作りたい",
    answers: ["createInventoryPickList"],
  },
  {
    id: "d2-15",
    axis: "D",
    text: "傷んだ荷物を提供元に送り返したい",
    answers: ["createInventoryVendorReturn"],
  },
  {
    id: "d2-16",
    axis: "D",
    text: "お客さんが怒っているので対応したい",
    answers: ["createSalesComplaint"],
  },
  {
    id: "d2-17",
    axis: "D",
    text: "無料の見本を送ってほしいと頼まれた",
    answers: ["createSalesSampleRequest"],
  },
  {
    id: "d2-18",
    axis: "D",
    text: "壊れた商品を直す約束を結びたい",
    answers: ["createSalesWarranty"],
  },
  {
    id: "d2-19",
    axis: "D",
    text: "常連客に特典を配りたい",
    answers: ["createSalesLoyaltyPoint"],
  },
  {
    id: "d2-20",
    axis: "D",
    text: "毎月自動で届く契約を結びたい",
    answers: ["createSalesSubscription"],
  },
  {
    id: "d2-21",
    axis: "D",
    text: "移動にかかったお金を会社に出してもらいたい",
    answers: ["createExpenseMileageClaim"],
  },
  {
    id: "d2-22",
    axis: "D",
    text: "取引先を持て成した支出を記録したい",
    answers: ["createExpenseEntertainment"],
  },
  {
    id: "d2-23",
    axis: "D",
    text: "転居に伴う出費を会社に見てもらいたい",
    answers: ["createExpenseRelocation"],
  },
  {
    id: "d2-24",
    axis: "D",
    text: "贈り物にかかったお金を記録したい",
    answers: ["createExpenseGift"],
  },
  {
    id: "d2-25",
    axis: "D",
    text: "新しい決済手段を会社に届け出たい",
    answers: ["createExpenseCorporateCard"],
  },
];
