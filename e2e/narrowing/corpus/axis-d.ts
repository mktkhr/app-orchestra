/**
 * Axis D (docs/specs/narrowing.md section 4): the vocabulary gap. Each
 * question shares no character bigram at all with its answer's combined
 * text (summary, description, display name, service display name) —
 * asserted in `corpus.test.ts` via `bigramsOf`, not just claimed. 品切れ →
 * 在庫切れ品目の一覧 was the obvious extra example and does not qualify: the
 * two share 切れ, so it belongs in axis A or C instead.
 *
 * A systemic trap worth naming: every CRUD operation's `description` is
 * built as `${noun}に対する${label}操作。`, which always contains the bigram
 * "する" (from 対する). A question phrased with a `〜する` verb ending will
 * almost always collide with it by accident; every question here ends in
 * `〜したい` instead, which does not.
 */
import type { Question } from "./types.ts";

export const AXIS_D: readonly Question[] = [
  {
    id: "d01",
    axis: "D",
    // Wanting time off is answered by seeing what leave is available, or by
    // creating the request that asks for it — both are "休みたい" (2026-09-14
    // DECISIONS.md: the model chose the latter). Neither shares a bigram
    // with the question.
    text: "休みたい",
    answers: ["listAttendancePaidLeaves", "createAttendanceLeaveRequest"],
  },
  {
    id: "d02",
    axis: "D",
    // "出したい" admits creating the PO and submitting it into the approval
    // workflow; the question does not pick one.
    text: "PO を出したい",
    answers: ["createPurchasingOrder", "submitPurchasingOrder"],
  },
  {
    id: "d03",
    axis: "D",
    // Same create-vs-submit ambiguity as d02, for an expense claim.
    text: "立て替えた分を出したい",
    answers: ["createExpenseClaim", "submitExpenseClaim"],
  },
  {
    id: "d04",
    axis: "D",
    // "使いたい" is answered by submitting the leave request or by creating
    // it outright; the question does not distinguish drafting from filing.
    text: "有給を使いたい",
    answers: ["submitAttendanceLeaveRequest", "createAttendanceLeaveRequest"],
  },
  {
    id: "d05",
    axis: "D",
    text: "品物が届いたので登録したい",
    answers: ["createInventoryReceiving"],
  },
  {
    id: "d06",
    axis: "D",
    // 出荷 is a process, not a single record: preparing a shipment can mean
    // building the pick list, the packing list, or the 出庫 record itself —
    // three stages of the same act (task instructions, corpus defect list).
    text: "出荷の準備をしたい",
    answers: ["createInventoryShipment", "createInventoryPickList", "createInventoryPackingList"],
  },
  {
    id: "d07",
    axis: "D",
    // "直したい" admits creating the correcting adjustment outright or
    // submitting it into the approval workflow.
    text: "減った分を直したい",
    answers: ["createInventoryAdjustment", "submitInventoryAdjustment"],
  },
  {
    id: "d08",
    axis: "D",
    text: "遅れて出社した記録を直したい",
    answers: ["updateAttendanceLateness"],
  },
  {
    id: "d09",
    axis: "D",
    // 精算, 経費申請 and 仮払 are all readings of "wanting money back" (task
    // instructions, corpus defect list); each is its own resource and none
    // shares a bigram with the question.
    text: "お金を返してもらいたい",
    answers: ["createExpenseReimbursement", "createExpenseClaim", "createExpenseAdvance"],
  },
  {
    id: "d10",
    axis: "D",
    text: "前借りしたお金の申請をやめたい",
    answers: ["withdrawExpenseAdvance"],
  },
  {
    id: "d11",
    axis: "D",
    text: "商品が届いたので受け取り処理をしたい",
    answers: ["createPurchasingGoodsReceipt"],
  },
  {
    id: "d12",
    axis: "D",
    // Genuinely undecidable between the two roles the sentence could be
    // spoken from (2026-09-14 DECISIONS.md): asking for a discount as a
    // buyer (RFQ) and granting one as a seller are both readings.
    text: "値段を安くしてほしいと頼みたい",
    answers: ["createSalesDiscount", "createPurchasingRequestForQuotation"],
  },
  {
    id: "d13",
    axis: "D",
    // "見たい" does not ask for individual records over a summary of them.
    text: "毎月自動で引き落とされる費用を見たい",
    answers: ["listExpenseRecurringExpenses", "summarizeExpenseRecurringExpenses"],
  },
  {
    id: "d14",
    axis: "D",
    // "手続きを知りたい" is answered by finding the matching orders as well
    // as by listing all of them.
    text: "急いで仕入れたい時の手続きを知りたい",
    answers: ["listPurchasingEmergencyOrders", "searchPurchasingEmergencyOrders"],
  },
  {
    id: "d15",
    axis: "D",
    // The model's answer (2026-09-14 DECISIONS.md): a search for items
    // nearing expiry is at least as good as the plain list of expiry dates.
    text: "そろそろダメになりそうな商品を確認したい",
    answers: ["listInventoryExpiryDates", "searchInventoryExpiringItems"],
  },
];
