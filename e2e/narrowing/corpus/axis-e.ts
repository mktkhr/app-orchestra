/**
 * Axis E (docs/specs/narrowing.md section 4): a decoy that is lexically
 * closer than the answer. Each question's `decoy` names a settings/master-
 * data operation whose summary literally reuses the question's words while
 * answering none of them — settings are named after the transactions they
 * configure, so a real catalogue is full of these.
 *
 * The shape that makes this axis real, not inverted: the person wants the
 * TRANSACTION RECORDS (never a settings operation — `corpus.test.ts` checks
 * `answers` structurally via `FixtureOperation.isSetting`), and asks using
 * the setting's longer, more specific noun (残業時間, not just 残業;
 * 倉庫容量, not just 倉庫) because that is the word that comes to mind, not
 * because they want the setting itself. `scoreOperation` then favours the
 * decoy because it carries every bigram of that longer noun, while the
 * answer — a plain list/get/approve of the underlying resource — only
 * carries the shorter one. Earlier drafts of this axis got this backwards:
 * a question like 「倉庫の容量設定を知りたい」 asks for the setting outright,
 * which makes the "decoy" the right answer — `corpus.test.ts`'s
 * `isSetting` check on `answers` exists specifically to catch that.
 *
 * The first question is the worked example from spec section 4, verbatim.
 */
import type { Question } from "./types.ts";

export const AXIS_E: readonly Question[] = [
  {
    id: "e01",
    axis: "E",
    text: "先月の残業時間",
    answers: ["listAttendanceRecords", "listAttendanceOvertimes"],
    decoy: "getAttendanceOvertimeThreshold",
  },
  {
    id: "e02",
    axis: "E",
    text: "関税率がかかってる取引を見たい",
    answers: ["listPurchasingCustomsDuties"],
    decoy: "updatePurchasingCustomsDutyRateSetting",
  },
  {
    id: "e03",
    axis: "E",
    text: "日当単価をもとに支給された分を確認したい",
    answers: ["listExpensePerDiems"],
    decoy: "getExpensePerDiemRateSetting",
  },
  {
    id: "e04",
    axis: "E",
    text: "販売手数料率で計算された分を確認したい",
    answers: ["listSalesCommissions"],
    decoy: "getSalesCommissionRateSetting",
  },
  {
    id: "e05",
    axis: "E",
    text: "倉庫容量がいっぱいな倉庫を知りたい",
    answers: ["listInventoryWarehouses"],
    decoy: "getInventoryWarehouseCapacitySetting",
  },
  {
    id: "e06",
    axis: "E",
    text: "与信限度額のしきい値に近づいている顧客を確認したい",
    answers: ["listSalesCreditLimits"],
    decoy: "getSalesCreditLimitThreshold",
  },
  {
    id: "e07",
    axis: "E",
    text: "発注点の閾値に近づいている品目を見たい",
    answers: ["listInventoryReorderPoints"],
    decoy: "updateInventoryReorderPointThreshold",
  },
  {
    id: "e08",
    axis: "E",
    text: "出張承認の上限に近い出張を確認したい",
    answers: ["listAttendanceBusinessTrips"],
    decoy: "updateAttendanceBusinessTripApprovalLimitSetting",
  },
  {
    id: "e09",
    axis: "E",
    text: "安全在庫の下限を下回りそうな品目を確認したい",
    answers: ["listInventorySafetyStocks"],
    decoy: "getInventorySafetyStockThreshold",
  },
  {
    id: "e10",
    axis: "E",
    text: "経費監査の対象になった記録を見たい",
    answers: ["listExpenseAuditLogs"],
    decoy: "getExpenseAuditSamplingRateSetting",
  },
];
