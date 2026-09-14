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
  { id: "d01", axis: "D", text: "休みたい", answers: ["listAttendancePaidLeaves"] },
  { id: "d02", axis: "D", text: "PO を出したい", answers: ["createPurchasingOrder"] },
  {
    id: "d03",
    axis: "D",
    text: "立て替えた分を出したい",
    answers: ["createExpenseClaim"],
  },
  {
    id: "d04",
    axis: "D",
    text: "有給を使いたい",
    answers: ["submitAttendanceLeaveRequest"],
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
    text: "出荷の準備をしたい",
    answers: ["createInventoryShipment"],
  },
  {
    id: "d07",
    axis: "D",
    text: "減った分を直したい",
    answers: ["createInventoryAdjustment"],
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
    text: "お金を返してもらいたい",
    answers: ["createExpenseReimbursement"],
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
    text: "値段を安くしてほしいと頼みたい",
    answers: ["createSalesDiscount"],
  },
  {
    id: "d13",
    axis: "D",
    text: "毎月自動で引き落とされる費用を見たい",
    answers: ["listExpenseRecurringExpenses"],
  },
  {
    id: "d14",
    axis: "D",
    text: "急いで仕入れたい時の手続きを知りたい",
    answers: ["listPurchasingEmergencyOrders"],
  },
  {
    id: "d15",
    axis: "D",
    text: "そろそろダメになりそうな商品を確認したい",
    answers: ["listInventoryExpiryDates"],
  },
];
