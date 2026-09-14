/**
 * Axis E (docs/specs/narrowing.md section 4): a decoy that is lexically
 * closer than the answer. Each question's `decoy` names a settings/master-
 * data operation whose summary literally reuses the question's words while
 * answering none of them — settings are named after the transactions they
 * configure, so a real catalogue is full of these. `corpus.test.ts` asserts
 * `scoreOperation` actually ranks the decoy above every answer, using the
 * same catalogue scope (`catalogOf(5)`) this file's operation ids come from.
 *
 * The first question is the worked example from spec section 4, verbatim.
 * The rest follow the same shape: a phrase that mirrors the decoy's own
 * qualifier words (しきい値 / 設定 / 率, not just the shared noun) so the
 * decoy's extra vocabulary — not the noun the answer also carries — is what
 * tips the score in its favour.
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
    text: "在庫調整の承認金額のしきい値を知りたい",
    answers: ["approveInventoryAdjustment"],
    decoy: "updateInventoryAdjustmentApprovalThreshold",
  },
  {
    id: "e03",
    axis: "E",
    text: "発注承認の金額のしきい値を知りたい",
    answers: ["listPurchasingApprovals"],
    decoy: "getPurchasingApprovalAmountThreshold",
  },
  {
    id: "e04",
    axis: "E",
    text: "精算の承認金額のしきい値を知りたい",
    answers: ["approveExpenseReimbursement"],
    decoy: "getExpenseReimbursementApprovalThreshold",
  },
  {
    id: "e05",
    axis: "E",
    text: "与信限度額のしきい値を知りたい",
    answers: ["listSalesCreditLimits"],
    decoy: "getSalesCreditLimitThreshold",
  },
  {
    id: "e06",
    axis: "E",
    text: "遅刻の許容時間の設定を知りたい",
    answers: ["listAttendanceLatenesss"],
    decoy: "updateAttendanceLatenessGraceMinutesSetting",
  },
  {
    id: "e07",
    axis: "E",
    text: "販売手数料率の設定を知りたい",
    answers: ["listSalesCommissions"],
    decoy: "getSalesCommissionRateSetting",
  },
  {
    id: "e08",
    axis: "E",
    text: "関税率の設定を知りたい",
    answers: ["listPurchasingCustomsDuties"],
    decoy: "updatePurchasingCustomsDutyRateSetting",
  },
  {
    id: "e09",
    axis: "E",
    text: "日当単価の設定を知りたい",
    answers: ["listExpensePerDiems"],
    decoy: "getExpensePerDiemRateSetting",
  },
  {
    id: "e10",
    axis: "E",
    text: "倉庫の容量設定を知りたい",
    answers: ["listInventoryWarehouses"],
    decoy: "getInventoryWarehouseCapacitySetting",
  },
];
