/**
 * Axis E extension (docs/specs/narrowing.md section 4): 25 more decoy
 * questions, same kind as `axis-e.ts` — each question's `decoy` names a
 * settings/master-data operation whose summary literally reuses the
 * question's words while answering none of them.
 *
 * Held in a separate file, and exposed only through `extensionQuestions()`
 * (`index.ts`), so the original 100 questions in `questions()` — and the
 * numbers already recorded against them — do not move.
 *
 * Same shape as `axis-e.ts`: the person wants the TRANSACTION RECORDS
 * (never a settings operation — `corpus-extension.test.ts` checks `answers`
 * structurally via `FixtureOperation.isSetting`), and asks using the
 * setting's longer, more specific noun because that is the word that comes
 * to mind, not because they want the setting itself. `scoreOperation` then
 * favours the decoy because it carries every bigram of that longer noun,
 * while the answer — a plain list/search/aggregate of the underlying
 * resource — only carries the shorter one; each decoy is asserted to
 * out-score every answer, not just claimed to.
 *
 * None of the twenty settings this file's decoys are drawn from
 * (per service) repeats a decoy `axis-e.ts` already used.
 */
import type { Question } from "./types.ts";

export const AXIS_E2: readonly Question[] = [
  {
    id: "e2-01",
    axis: "E",
    // "承認金額のしきい値に近い" reuses the setting's own wording; the person
    // wants the adjustments themselves, drafted or already summarised.
    text: "承認金額のしきい値に近い在庫調整を確認したい",
    answers: ["listInventoryAdjustments", "summarizeInventoryAdjustments"],
    decoy: "updateInventoryAdjustmentApprovalThreshold",
  },
  {
    id: "e2-02",
    axis: "E",
    text: "使用期限アラートのリードタイムに近づいている商品を知りたい",
    answers: ["listInventoryExpiryDates", "searchInventoryExpiringItems"],
    decoy: "getInventoryExpiryAlertLeadTimeSetting",
  },
  {
    id: "e2-03",
    axis: "E",
    text: "品質検査の合格基準に届かなかった品目を確認したい",
    answers: ["listInventoryQualityInspections", "summarizeInventoryQualityInspections"],
    decoy: "updateInventoryQualityInspectionThreshold",
  },
  {
    id: "e2-04",
    axis: "E",
    text: "コンテナ容量がいっぱいになっているコンテナを知りたい",
    answers: ["listInventoryContainers"],
    decoy: "updateInventoryContainerCapacitySetting",
  },
  {
    id: "e2-05",
    axis: "E",
    text: "パレット積載量に近づいているパレットを知りたい",
    answers: ["listInventoryPallets"],
    decoy: "getInventoryPalletCapacitySetting",
  },
  {
    id: "e2-06",
    axis: "E",
    // "許容誤差を超えている見込み" admits the plain list of forecasts and the
    // aggregate over them equally; the question does not pick one.
    text: "販売予測の許容誤差を超えている見込みを確認したい",
    answers: ["listSalesForecasts", "aggregateSalesForecasts"],
    decoy: "getSalesForecastAccuracyThreshold",
  },
  {
    id: "e2-07",
    axis: "E",
    text: "保証期間がもうすぐ切れる保証を確認したい",
    answers: ["listSalesWarranties"],
    decoy: "getSalesWarrantyPeriodDefaultSetting",
  },
  {
    id: "e2-08",
    axis: "E",
    text: "ポイント付与率をもとに貯まったポイントを確認したい",
    answers: ["listSalesLoyaltyPoints"],
    decoy: "updateSalesLoyaltyPointRateSetting",
  },
  {
    id: "e2-09",
    axis: "E",
    text: "見積有効期限が近づいている見積を確認したい",
    answers: ["listSalesQuotations", "summarizeSalesQuotations"],
    decoy: "getSalesQuotationValidityPeriodSetting",
  },
  {
    id: "e2-10",
    axis: "E",
    text: "請求書の支払期日が近づいている請求書を確認したい",
    answers: ["listSalesInvoices", "summarizeSalesInvoices"],
    decoy: "updateSalesInvoiceDueDateDefaultSetting",
  },
  {
    id: "e2-11",
    axis: "E",
    // "発注承認の金額のしきい値に近づいている発注" is undecided between the
    // approval records and the orders they gate — both are defensible, as
    // in `axis-d.ts`'s d12.
    text: "発注承認の金額のしきい値に近づいている発注を確認したい",
    answers: ["listPurchasingApprovals", "listPurchasingOrders"],
    decoy: "getPurchasingApprovalAmountThreshold",
  },
  {
    id: "e2-12",
    axis: "E",
    text: "受入検査ルールに基づいて検査した記録を確認したい",
    answers: ["listPurchasingReceivingInspections"],
    decoy: "getPurchasingReceivingInspectionRuleSetting",
  },
  {
    id: "e2-13",
    axis: "E",
    text: "緊急発注の許容金額に近づいている発注を確認したい",
    answers: ["listPurchasingEmergencyOrders", "searchPurchasingEmergencyOrders"],
    decoy: "updatePurchasingEmergencyOrderThreshold",
  },
  {
    id: "e2-14",
    axis: "E",
    text: "欠品発注の通知設定が出ている発注を確認したい",
    answers: ["listPurchasingBackorders", "searchPurchasingBackorders"],
    decoy: "updatePurchasingBackorderNotificationSetting",
  },
  {
    id: "e2-15",
    axis: "E",
    text: "契約更新通知の時期が近づいている契約を確認したい",
    answers: ["listPurchasingContracts", "aggregatePurchasingContracts"],
    decoy: "getPurchasingContractRenewalNoticeSetting",
  },
  {
    id: "e2-16",
    axis: "E",
    // The fixture's `pluralOf` produces "Latenesss" (triple s) for
    // "Lateness"; `listAttendanceLatenesss` is the operation the fixture
    // actually serves, not a typo introduced here.
    text: "遅刻許容時間を超えて遅れた記録を確認したい",
    answers: ["listAttendanceLatenesss", "summarizeAttendanceLatenesses"],
    decoy: "updateAttendanceLatenessGraceMinutesSetting",
  },
  {
    id: "e2-17",
    axis: "E",
    text: "有給休暇付与ルールに基づいてもらえる休暇を確認したい",
    answers: ["listAttendancePaidLeaves", "searchAttendancePaidLeaves"],
    decoy: "getAttendancePaidLeaveGrantRuleSetting",
  },
  {
    id: "e2-18",
    axis: "E",
    text: "通勤手当率をもとに支給される通勤経路を確認したい",
    answers: ["listAttendanceCommutes"],
    decoy: "getAttendanceCommuteAllowanceRateSetting",
  },
  {
    id: "e2-19",
    axis: "E",
    text: "人事考課サイクルに従って行われた考課を確認したい",
    answers: ["listAttendanceAnnualReviews"],
    decoy: "getAttendanceAnnualReviewCycleSetting",
  },
  {
    id: "e2-20",
    axis: "E",
    text: "健康診断の実施間隔をもとに受診履歴を確認したい",
    answers: ["listAttendanceHealthCheckups", "searchAttendanceHealthCheckups"],
    decoy: "getAttendanceHealthCheckupIntervalSetting",
  },
  {
    id: "e2-21",
    axis: "E",
    text: "精算承認の金額のしきい値に近づいている精算を確認したい",
    answers: ["listExpenseReimbursements", "aggregateExpenseReimbursements"],
    decoy: "getExpenseReimbursementApprovalThreshold",
  },
  {
    id: "e2-22",
    axis: "E",
    text: "仮払限度額に近づいている仮払を確認したい",
    answers: ["listExpenseAdvances", "summarizeExpenseAdvances"],
    decoy: "updateExpenseAdvanceLimitThreshold",
  },
  {
    id: "e2-23",
    axis: "E",
    text: "接待交際費の上限に近づいている支出を確認したい",
    answers: ["listExpenseEntertainments", "summarizeExpenseEntertainments"],
    decoy: "updateExpenseEntertainmentLimitThreshold",
  },
  {
    id: "e2-24",
    axis: "E",
    // "法人カード限度額に近づいている利用" is read as the card's usage
    // records (the transactions), not the card record itself.
    text: "法人カード限度額に近づいているカード利用を確認したい",
    answers: ["listExpenseCardTransactions", "summarizeExpenseCardTransactions"],
    decoy: "getExpenseCorporateCardLimitSetting",
  },
  {
    id: "e2-25",
    axis: "E",
    text: "転勤費用の上限に近づいている費用を確認したい",
    answers: ["listExpenseRelocations", "summarizeExpenseRelocations"],
    decoy: "updateExpenseRelocationLimitSetting",
  },
];
