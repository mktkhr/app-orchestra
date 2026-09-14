/**
 * 購買 (purchasing, 発注) — 200 operations. Shares "order" (受注/発注),
 * "line" (受注明細/発注明細/経費明細), "approval" (経費承認/発注承認/勤怠承認)
 * and "partner" (取引先) with other services — axis B.
 */
import { purchasingExamples } from "./examples-purchasing.ts";
import { ALL_VERBS, pluralOf } from "./naming.ts";
import type {
  Aggregate,
  Resource,
  ServiceFixture,
  Setting,
  Verb,
  Workflow,
  WorkflowAction,
} from "./types.ts";

type ResourceExamples = Partial<Readonly<Record<Verb, readonly string[]>>>;
type WorkflowExamples = Partial<Readonly<Record<WorkflowAction, readonly string[]>>>;

/**
 * Index-signature views onto `purchasingExamples`, so an id can be looked up
 * dynamically (`Record<string, ...>`, not a fixed literal union) without a
 * type assertion - a plain-typed variable declaration is enough because the
 * literal object is structurally assignable to it.
 */
const resourceExamples: Readonly<Record<string, ResourceExamples>> = purchasingExamples.resources;
const aggregateExamples: Readonly<Record<string, readonly string[]>> =
  purchasingExamples.aggregates;
const settingExamples: Readonly<Record<string, readonly string[]>> = purchasingExamples.settings;
const workflowExamples: Readonly<Record<string, WorkflowExamples>> = purchasingExamples.workflows;

function r(id: string, noun: string, opts?: { group?: string; shared?: string }): Resource {
  const examples = resourceExamples[id];

  return {
    id,
    plural: pluralOf(id),
    noun,
    verbs: ALL_VERBS,
    ...(opts?.group === undefined ? {} : { group: opts.group }),
    ...(opts?.shared === undefined ? {} : { shared: opts.shared }),
    ...(examples === undefined ? {} : { examples }),
  };
}

const resources: readonly Resource[] = [
  // axis C group "order"
  r("Order", "発注", { group: "order", shared: "order" }),
  r("OrderLine", "発注明細", { group: "order", shared: "line" }),
  r("PurchaseRequest", "購買依頼", { group: "order" }),
  r("GoodsReceipt", "検収", { group: "order" }),
  r("Invoice", "仕入請求書", { group: "order" }),
  // axis C group "supplier"
  r("Supplier", "仕入先", { group: "supplier" }),
  r("Partner", "取引先", { group: "supplier", shared: "partner" }),
  r("Contract", "取引契約", { group: "supplier" }),
  r("SupplierEvaluation", "仕入先評価", { group: "supplier" }),
  // axis C group "approval-flow"
  r("Approval", "発注承認", { group: "approval-flow", shared: "approval" }),
  r("Budget", "予算", { group: "approval-flow" }),
  r("CostCenter", "コストセンター", { group: "approval-flow" }),
  r("SpendingLimit", "支出限度額", { group: "approval-flow" }),
  r("Delegation", "決裁委任", { group: "approval-flow" }),
  // ungrouped
  r("RequestForQuotation", "見積依頼"),
  r("Bid", "入札"),
  r("PurchaseCatalog", "購買カタログ"),
  r("Blanket", "単価契約"),
  r("SupplierReturn", "仕入先向け返品"),
  r("ReceivingInspection", "受入検査"),
  r("Backorder", "欠品発注"),
  r("DeliveryLeadTime", "納期"),
  r("ShippingTerm", "貿易条件"),
  r("ImportDeclaration", "輸入申告"),
  r("CustomsDuty", "関税"),
  r("SupplierScorecard", "仕入先スコアカード"),
  r("PaymentSchedule", "支払スケジュール"),
  r("Consignment", "預託購買"),
  r("DirectShip", "直送"),
  r("EmergencyOrder", "緊急発注"),
];

const aggregates: readonly Aggregate[] = [
  { id: "Orders", kind: "search", noun: "発注" },
  { id: "OrderLines", kind: "search", noun: "発注明細" },
  { id: "PurchaseRequests", kind: "summarize", noun: "購買依頼" },
  { id: "GoodsReceipts", kind: "summarize", noun: "検収" },
  { id: "Invoices", kind: "summarize", noun: "仕入請求書" },
  { id: "Suppliers", kind: "search", noun: "仕入先" },
  { id: "Partners", kind: "search", noun: "取引先" },
  { id: "Contracts", kind: "aggregate", noun: "取引契約" },
  { id: "Budgets", kind: "aggregate", noun: "予算" },
  { id: "CostCenters", kind: "summarize", noun: "コストセンター" },
  { id: "SpendingLimits", kind: "aggregate", noun: "支出限度額" },
  { id: "Bids", kind: "search", noun: "入札" },
  { id: "RequestForQuotations", kind: "search", noun: "見積依頼" },
  { id: "Backorders", kind: "search", noun: "欠品発注" },
  { id: "CustomsDuties", kind: "aggregate", noun: "関税" },
  { id: "SupplierScorecards", kind: "summarize", noun: "仕入先スコアカード" },
  { id: "PaymentSchedules", kind: "summarize", noun: "支払スケジュール" },
  { id: "Consignments", kind: "summarize", noun: "預託購買" },
  { id: "EmergencyOrders", kind: "search", noun: "緊急発注" },
  { id: "Blankets", kind: "summarize", noun: "単価契約" },
];

const settings: readonly Setting[] = [
  {
    id: "ApprovalAmountThreshold",
    verb: "get",
    summary: "発注承認の金額しきい値を取得",
    displayName: "発注承認しきい値",
  },
  {
    id: "BudgetLimitSetting",
    verb: "update",
    summary: "予算上限の設定を更新",
    displayName: "予算上限設定",
  },
  {
    id: "CostCenterCategory",
    verb: "list",
    summary: "コストセンター区分の一覧",
    displayName: "コストセンター区分一覧",
  },
  {
    id: "SupplierEvaluationCriteriaSetting",
    verb: "get",
    summary: "仕入先評価基準の設定を取得",
    displayName: "仕入先評価基準設定",
  },
  {
    id: "LeadTimeDefaultSetting",
    verb: "update",
    summary: "標準納期の設定を更新",
    displayName: "標準納期設定",
  },
  {
    id: "RFQCategory",
    verb: "list",
    summary: "見積依頼区分の一覧",
    displayName: "見積依頼区分一覧",
  },
  {
    id: "ContractRenewalNoticeSetting",
    verb: "get",
    summary: "契約更新通知の設定を取得",
    displayName: "契約更新通知設定",
  },
  {
    id: "SpendingLimitThreshold",
    verb: "update",
    summary: "支出限度額の設定を更新",
    displayName: "支出限度額設定",
  },
  { id: "BidCategory", verb: "list", summary: "入札区分の一覧", displayName: "入札区分一覧" },
  {
    id: "ReceivingInspectionRuleSetting",
    verb: "get",
    summary: "受入検査ルールの設定を取得",
    displayName: "受入検査ルール設定",
  },
  {
    id: "CustomsDutyRateSetting",
    verb: "update",
    summary: "関税率の設定を更新",
    displayName: "関税率設定",
  },
  {
    id: "ConsignmentCategory",
    verb: "list",
    summary: "預託購買区分の一覧",
    displayName: "預託購買区分一覧",
  },
  {
    id: "ScorecardWeightSetting",
    verb: "get",
    summary: "スコアカード重み設定を取得",
    displayName: "スコアカード重み設定",
  },
  {
    id: "PaymentScheduleDefaultSetting",
    verb: "update",
    summary: "支払スケジュール既定値の設定を更新",
    displayName: "支払スケジュール既定値設定",
  },
  {
    id: "PurchaseCatalogCategory",
    verb: "list",
    summary: "購買カタログ区分の一覧",
    displayName: "購買カタログ区分一覧",
  },
  {
    id: "DelegationLimitSetting",
    verb: "get",
    summary: "決裁委任限度額の設定を取得",
    displayName: "決裁委任限度額設定",
  },
  {
    id: "EmergencyOrderThreshold",
    verb: "update",
    summary: "緊急発注の許容金額を更新",
    displayName: "緊急発注許容金額",
  },
  {
    id: "ShippingTermCategory",
    verb: "list",
    summary: "貿易条件区分の一覧",
    displayName: "貿易条件区分一覧",
  },
  {
    id: "BlanketContractDefaultSetting",
    verb: "get",
    summary: "単価契約の既定条件を取得",
    displayName: "単価契約既定条件設定",
  },
  {
    id: "BackorderNotificationSetting",
    verb: "update",
    summary: "欠品発注の通知設定を更新",
    displayName: "欠品発注通知設定",
  },
];

const workflows: readonly Workflow[] = [
  { id: "Order", noun: "発注", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "PurchaseRequest", noun: "購買依頼", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "Contract", noun: "取引契約", actions: ["submit", "approve"] },
];

function withAggregateExamples(list: readonly Aggregate[]): readonly Aggregate[] {
  return list.map((aggregate) => {
    const examples = aggregateExamples[aggregate.id];

    return { ...aggregate, ...(examples === undefined ? {} : { examples }) };
  });
}

function withSettingExamples(list: readonly Setting[]): readonly Setting[] {
  return list.map((setting) => {
    const examples = settingExamples[setting.id];

    return { ...setting, ...(examples === undefined ? {} : { examples }) };
  });
}

function withWorkflowExamples(list: readonly Workflow[]): readonly Workflow[] {
  return list.map((workflow) => {
    const examples = workflowExamples[workflow.id];

    return { ...workflow, ...(examples === undefined ? {} : { examples }) };
  });
}

export const purchasing: ServiceFixture = {
  name: "purchasing",
  displayName: "購買管理",
  resources,
  aggregates: withAggregateExamples(aggregates),
  settings: withSettingExamples(settings),
  workflows: withWorkflowExamples(workflows),
};
