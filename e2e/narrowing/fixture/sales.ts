/**
 * 販売 (sales, 受注) — 200 operations. Shares "order" (受注/発注), "line"
 * (受注明細/発注明細/経費明細) and "partner" (取引先) with other services —
 * axis B. See docs/specs/narrowing.md section 3.
 */
import { examplesSales } from "./examples-sales.ts";
import { ALL_VERBS, pluralOf } from "./naming.ts";
import type { Aggregate, Resource, ServiceFixture, Setting, Workflow } from "./types.ts";

function r(
  id: keyof typeof examplesSales.resources,
  noun: string,
  opts?: { group?: string; shared?: string },
): Resource {
  return {
    id,
    plural: pluralOf(id),
    noun,
    verbs: ALL_VERBS,
    examples: examplesSales.resources[id],
    ...(opts?.group === undefined ? {} : { group: opts.group }),
    ...(opts?.shared === undefined ? {} : { shared: opts.shared }),
  };
}

function a(
  id: keyof typeof examplesSales.aggregates,
  kind: Aggregate["kind"],
  noun: string,
): Aggregate {
  return { id, kind, noun, examples: examplesSales.aggregates[id] };
}

function st(
  id: keyof typeof examplesSales.settings,
  verb: Setting["verb"],
  summary: string,
  displayName: string,
): Setting {
  return { id, verb, summary, displayName, examples: examplesSales.settings[id] };
}

function wf(
  id: keyof typeof examplesSales.workflows,
  noun: string,
  actions: Workflow["actions"],
): Workflow {
  return { id, noun, actions, examples: examplesSales.workflows[id] };
}

const resources: readonly Resource[] = [
  // axis C group "order"; axis B: Order and OrderLine also collapse displayName to a shared noun.
  r("Order", "受注", { group: "order", shared: "order" }),
  r("OrderLine", "受注明細", { group: "order", shared: "line" }),
  r("Quotation", "見積", { group: "order" }),
  r("DeliveryNote", "納品書", { group: "order" }),
  r("Invoice", "請求書", { group: "order" }),
  // axis C group "customer"
  r("Customer", "顧客", { group: "customer" }),
  r("Partner", "取引先", { group: "customer", shared: "partner" }),
  r("Contact", "取引先担当者", { group: "customer" }),
  r("CreditLimit", "与信限度額", { group: "customer" }),
  // axis C group "pricing"
  r("PriceList", "価格表", { group: "pricing" }),
  r("Discount", "値引", { group: "pricing" }),
  r("Campaign", "販促キャンペーン", { group: "pricing" }),
  r("Coupon", "クーポン", { group: "pricing" }),
  r("Commission", "販売手数料", { group: "pricing" }),
  // ungrouped
  r("SalesRep", "営業担当者"),
  r("Territory", "営業エリア"),
  r("Forecast", "販売予測"),
  r("ReturnOrder", "返品"),
  r("Backorder", "欠品受注"),
  r("ShippingAddress", "配送先"),
  r("PaymentTerm", "支払条件"),
  r("SalesChannel", "販売チャネル"),
  r("Warranty", "保証"),
  r("ServiceContract", "保守契約"),
  r("Complaint", "クレーム"),
  r("LoyaltyPoint", "ポイント"),
  r("GiftCard", "ギフトカード"),
  r("Subscription", "定期購買"),
  r("Bundle", "セット商品"),
  r("SampleRequest", "サンプル依頼"),
];

const aggregates: readonly Aggregate[] = [
  a("Orders", "search", "受注"),
  a("OrderLines", "search", "受注明細"),
  a("Quotations", "summarize", "見積"),
  a("Invoices", "summarize", "請求書"),
  a("DeliveryNotes", "search", "納品書"),
  a("Customers", "search", "顧客"),
  a("Partners", "search", "取引先"),
  a("CreditLimits", "aggregate", "与信限度額"),
  a("PriceLists", "search", "価格表"),
  a("Discounts", "summarize", "値引"),
  a("Campaigns", "summarize", "販促キャンペーン"),
  a("Commissions", "aggregate", "販売手数料"),
  a("SalesReps", "search", "営業担当者"),
  a("Territories", "summarize", "営業エリア"),
  a("Forecasts", "aggregate", "販売予測"),
  a("ReturnOrders", "summarize", "返品"),
  a("Backorders", "search", "欠品受注"),
  a("Complaints", "search", "クレーム"),
  a("Subscriptions", "summarize", "定期購買"),
  a("Bundles", "search", "セット商品"),
];

const settings: readonly Setting[] = [
  st("CreditLimitThreshold", "get", "与信限度額のしきい値を取得", "与信限度額設定"),
  st("DiscountApprovalThreshold", "update", "値引承認の金額しきい値を更新", "値引承認しきい値"),
  st("PriceListCategory", "list", "価格表区分の一覧", "価格表区分一覧"),
  st("CommissionRateSetting", "get", "販売手数料率の設定を取得", "販売手数料率設定"),
  st("TerritoryDefaultSetting", "update", "営業エリア既定値の設定を更新", "営業エリア既定値設定"),
  st("CampaignCategory", "list", "販促キャンペーン区分の一覧", "キャンペーン区分一覧"),
  st("ForecastAccuracyThreshold", "get", "販売予測の許容誤差設定を取得", "販売予測誤差設定"),
  st("BackorderNotificationSetting", "update", "欠品受注の通知設定を更新", "欠品受注通知設定"),
  st("PaymentTermCategory", "list", "支払条件区分の一覧", "支払条件区分一覧"),
  st("WarrantyPeriodDefaultSetting", "get", "保証期間の既定値設定を取得", "保証期間既定値設定"),
  st("LoyaltyPointRateSetting", "update", "ポイント付与率の設定を更新", "ポイント付与率設定"),
  st("GiftCardCategory", "list", "ギフトカード区分の一覧", "ギフトカード区分一覧"),
  st(
    "SubscriptionRenewalNoticeSetting",
    "get",
    "定期購買更新通知の設定を取得",
    "定期購買更新通知設定",
  ),
  st("ReturnReasonCategorySetting", "update", "返品理由区分の設定を更新", "返品理由区分設定"),
  st("SalesChannelCategory", "list", "販売チャネル区分の一覧", "販売チャネル区分一覧"),
  st("QuotationValidityPeriodSetting", "get", "見積有効期限の設定を取得", "見積有効期限設定"),
  st(
    "InvoiceDueDateDefaultSetting",
    "update",
    "請求書支払期日の既定値を更新",
    "請求書支払期日設定",
  ),
  st("ComplaintCategory", "list", "クレーム区分の一覧", "クレーム区分一覧"),
  st("BundleDiscountRuleSetting", "get", "セット商品割引ルールの設定を取得", "セット商品割引設定"),
  st("SampleRequestLimitSetting", "update", "サンプル依頼の上限設定を更新", "サンプル依頼上限設定"),
];

const workflows: readonly Workflow[] = [
  wf("Order", "受注", ["submit", "approve", "reject", "withdraw"]),
  wf("Quotation", "見積", ["submit", "approve", "reject", "withdraw"]),
  wf("ReturnOrder", "返品", ["submit", "approve"]),
];

export const sales: ServiceFixture = {
  name: "sales",
  displayName: "販売管理",
  resources,
  aggregates,
  settings,
  workflows,
};
