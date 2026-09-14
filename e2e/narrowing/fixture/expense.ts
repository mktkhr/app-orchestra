/**
 * 経費 (expense) — 200 operations. Shares "line" (受注明細/発注明細/経費明細),
 * "approval" (経費承認/発注承認/勤怠承認) and "employee" (社員) with sales,
 * purchasing and attendance — axis B.
 */
import { ALL_VERBS, pluralOf } from "./naming.ts";
import type { Aggregate, Resource, ServiceFixture, Setting, Workflow } from "./types.ts";

function r(id: string, noun: string, opts?: { group?: string; shared?: string }): Resource {
  return {
    id,
    plural: pluralOf(id),
    noun,
    verbs: ALL_VERBS,
    ...(opts?.group === undefined ? {} : { group: opts.group }),
    ...(opts?.shared === undefined ? {} : { shared: opts.shared }),
  };
}

const resources: readonly Resource[] = [
  // axis C group "claim"
  r("ExpenseClaim", "経費申請", { group: "claim" }),
  r("ExpenseLine", "経費明細", { group: "claim", shared: "line" }),
  r("Receipt", "領収書", { group: "claim" }),
  r("TravelExpense", "出張旅費", { group: "claim" }),
  r("Reimbursement", "精算", { group: "claim" }),
  // axis C group "approval-flow"
  r("Approval", "経費承認", { group: "approval-flow", shared: "approval" }),
  r("Budget", "予算", { group: "approval-flow" }),
  r("CostCategory", "費目", { group: "approval-flow" }),
  r("SpendingLimit", "経費限度額", { group: "approval-flow" }),
  // axis C group "card"
  r("CorporateCard", "法人カード", { group: "card" }),
  r("CardTransaction", "カード利用明細", { group: "card" }),
  r("Advance", "仮払", { group: "card" }),
  r("Settlement", "清算", { group: "card" }),
  r("MileageClaim", "通勤費申請", { group: "card" }),
  // ungrouped
  r("Employee", "社員", { shared: "employee" }),
  r("Department", "所属部署"),
  r("Vendor", "支払先"),
  r("TaxRate", "税率"),
  r("Currency", "通貨"),
  r("ExchangeRate", "為替レート"),
  r("Invoice", "請求書"),
  r("Payment", "支払"),
  r("Entertainment", "接待交際費"),
  r("Subscription", "サブスク利用"),
  r("RecurringExpense", "定期経費"),
  r("Allowance", "手当"),
  r("PerDiem", "日当"),
  r("Relocation", "転勤費用"),
  r("Gift", "贈答品"),
  r("AuditLog", "経費監査ログ"),
];

const aggregates: readonly Aggregate[] = [
  { id: "ExpenseClaims", kind: "search", noun: "経費申請" },
  { id: "ExpenseLines", kind: "search", noun: "経費明細" },
  { id: "Receipts", kind: "summarize", noun: "領収書" },
  { id: "TravelExpenses", kind: "summarize", noun: "出張旅費" },
  { id: "Reimbursements", kind: "aggregate", noun: "精算" },
  { id: "Budgets", kind: "aggregate", noun: "予算" },
  { id: "CostCategories", kind: "summarize", noun: "費目" },
  { id: "SpendingLimits", kind: "aggregate", noun: "経費限度額" },
  { id: "CorporateCards", kind: "search", noun: "法人カード" },
  { id: "CardTransactions", kind: "summarize", noun: "カード利用明細" },
  { id: "Advances", kind: "summarize", noun: "仮払" },
  { id: "Settlements", kind: "aggregate", noun: "清算" },
  { id: "MileageClaims", kind: "search", noun: "通勤費申請" },
  { id: "Vendors", kind: "search", noun: "支払先" },
  { id: "Payments", kind: "summarize", noun: "支払" },
  { id: "Entertainments", kind: "summarize", noun: "接待交際費" },
  { id: "RecurringExpenses", kind: "summarize", noun: "定期経費" },
  { id: "Allowances", kind: "aggregate", noun: "手当" },
  { id: "PerDiems", kind: "aggregate", noun: "日当" },
  { id: "Relocations", kind: "summarize", noun: "転勤費用" },
];

const settings: readonly Setting[] = [
  {
    id: "ReimbursementApprovalThreshold",
    verb: "get",
    summary: "精算承認の金額しきい値を取得",
    displayName: "精算承認しきい値",
  },
  {
    id: "MileageRateSetting",
    verb: "update",
    summary: "通勤費単価の設定を更新",
    displayName: "通勤費単価設定",
  },
  {
    id: "CostCategoryGroup",
    verb: "list",
    summary: "費目グループの一覧",
    displayName: "費目グループ一覧",
  },
  {
    id: "PerDiemRateSetting",
    verb: "get",
    summary: "日当単価の設定を取得",
    displayName: "日当単価設定",
  },
  {
    id: "AdvanceLimitThreshold",
    verb: "update",
    summary: "仮払限度額の設定を更新",
    displayName: "仮払限度額設定",
  },
  { id: "CurrencyCategory", verb: "list", summary: "通貨区分の一覧", displayName: "通貨区分一覧" },
  {
    id: "ExchangeRateUpdateFrequencySetting",
    verb: "get",
    summary: "為替レート更新頻度の設定を取得",
    displayName: "為替レート更新頻度設定",
  },
  {
    id: "EntertainmentLimitThreshold",
    verb: "update",
    summary: "接待交際費の上限設定を更新",
    displayName: "接待交際費上限設定",
  },
  { id: "TaxRateCategory", verb: "list", summary: "税率区分の一覧", displayName: "税率区分一覧" },
  {
    id: "CorporateCardLimitSetting",
    verb: "get",
    summary: "法人カード限度額の設定を取得",
    displayName: "法人カード限度額設定",
  },
  {
    id: "AllowanceDefaultRateSetting",
    verb: "update",
    summary: "手当既定率の設定を更新",
    displayName: "手当既定率設定",
  },
  {
    id: "VendorCategory",
    verb: "list",
    summary: "支払先区分の一覧",
    displayName: "支払先区分一覧",
  },
  {
    id: "RecurringExpenseIntervalSetting",
    verb: "get",
    summary: "定期経費の実行間隔設定を取得",
    displayName: "定期経費間隔設定",
  },
  {
    id: "RelocationLimitSetting",
    verb: "update",
    summary: "転勤費用の上限設定を更新",
    displayName: "転勤費用上限設定",
  },
  { id: "GiftCategory", verb: "list", summary: "贈答品区分の一覧", displayName: "贈答品区分一覧" },
  {
    id: "AuditSamplingRateSetting",
    verb: "get",
    summary: "経費監査の抽出率設定を取得",
    displayName: "経費監査抽出率設定",
  },
  {
    id: "ApprovalRouteDefaultSetting",
    verb: "update",
    summary: "経費承認ルートの既定設定を更新",
    displayName: "経費承認ルート設定",
  },
  { id: "PaymentCategory", verb: "list", summary: "支払区分の一覧", displayName: "支払区分一覧" },
  {
    id: "SettlementCycleSetting",
    verb: "get",
    summary: "清算サイクルの設定を取得",
    displayName: "清算サイクル設定",
  },
  {
    id: "ReceiptRequiredThreshold",
    verb: "update",
    summary: "領収書添付必須金額の設定を更新",
    displayName: "領収書添付必須金額設定",
  },
];

const workflows: readonly Workflow[] = [
  { id: "ExpenseClaim", noun: "経費申請", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "Advance", noun: "仮払", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "Reimbursement", noun: "精算", actions: ["submit", "approve"] },
];

export const expense: ServiceFixture = {
  name: "expense",
  displayName: "経費管理",
  resources,
  aggregates,
  settings,
  workflows,
};
