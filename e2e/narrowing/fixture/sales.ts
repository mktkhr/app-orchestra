/**
 * 販売 (sales, 受注) — 200 operations. Shares "order" (受注/発注), "line"
 * (受注明細/発注明細/経費明細) and "partner" (取引先) with other services —
 * axis B. See docs/specs/narrowing.md section 3.
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
  { id: "Orders", kind: "search", noun: "受注" },
  { id: "OrderLines", kind: "search", noun: "受注明細" },
  { id: "Quotations", kind: "summarize", noun: "見積" },
  { id: "Invoices", kind: "summarize", noun: "請求書" },
  { id: "DeliveryNotes", kind: "search", noun: "納品書" },
  { id: "Customers", kind: "search", noun: "顧客" },
  { id: "Partners", kind: "search", noun: "取引先" },
  { id: "CreditLimits", kind: "aggregate", noun: "与信限度額" },
  { id: "PriceLists", kind: "search", noun: "価格表" },
  { id: "Discounts", kind: "summarize", noun: "値引" },
  { id: "Campaigns", kind: "summarize", noun: "販促キャンペーン" },
  { id: "Commissions", kind: "aggregate", noun: "販売手数料" },
  { id: "SalesReps", kind: "search", noun: "営業担当者" },
  { id: "Territories", kind: "summarize", noun: "営業エリア" },
  { id: "Forecasts", kind: "aggregate", noun: "販売予測" },
  { id: "ReturnOrders", kind: "summarize", noun: "返品" },
  { id: "Backorders", kind: "search", noun: "欠品受注" },
  { id: "Complaints", kind: "search", noun: "クレーム" },
  { id: "Subscriptions", kind: "summarize", noun: "定期購買" },
  { id: "Bundles", kind: "search", noun: "セット商品" },
];

const settings: readonly Setting[] = [
  {
    id: "CreditLimitThreshold",
    verb: "get",
    summary: "与信限度額のしきい値を取得",
    displayName: "与信限度額設定",
  },
  {
    id: "DiscountApprovalThreshold",
    verb: "update",
    summary: "値引承認の金額しきい値を更新",
    displayName: "値引承認しきい値",
  },
  {
    id: "PriceListCategory",
    verb: "list",
    summary: "価格表区分の一覧",
    displayName: "価格表区分一覧",
  },
  {
    id: "CommissionRateSetting",
    verb: "get",
    summary: "販売手数料率の設定を取得",
    displayName: "販売手数料率設定",
  },
  {
    id: "TerritoryDefaultSetting",
    verb: "update",
    summary: "営業エリア既定値の設定を更新",
    displayName: "営業エリア既定値設定",
  },
  {
    id: "CampaignCategory",
    verb: "list",
    summary: "販促キャンペーン区分の一覧",
    displayName: "キャンペーン区分一覧",
  },
  {
    id: "ForecastAccuracyThreshold",
    verb: "get",
    summary: "販売予測の許容誤差設定を取得",
    displayName: "販売予測誤差設定",
  },
  {
    id: "BackorderNotificationSetting",
    verb: "update",
    summary: "欠品受注の通知設定を更新",
    displayName: "欠品受注通知設定",
  },
  {
    id: "PaymentTermCategory",
    verb: "list",
    summary: "支払条件区分の一覧",
    displayName: "支払条件区分一覧",
  },
  {
    id: "WarrantyPeriodDefaultSetting",
    verb: "get",
    summary: "保証期間の既定値設定を取得",
    displayName: "保証期間既定値設定",
  },
  {
    id: "LoyaltyPointRateSetting",
    verb: "update",
    summary: "ポイント付与率の設定を更新",
    displayName: "ポイント付与率設定",
  },
  {
    id: "GiftCardCategory",
    verb: "list",
    summary: "ギフトカード区分の一覧",
    displayName: "ギフトカード区分一覧",
  },
  {
    id: "SubscriptionRenewalNoticeSetting",
    verb: "get",
    summary: "定期購買更新通知の設定を取得",
    displayName: "定期購買更新通知設定",
  },
  {
    id: "ReturnReasonCategorySetting",
    verb: "update",
    summary: "返品理由区分の設定を更新",
    displayName: "返品理由区分設定",
  },
  {
    id: "SalesChannelCategory",
    verb: "list",
    summary: "販売チャネル区分の一覧",
    displayName: "販売チャネル区分一覧",
  },
  {
    id: "QuotationValidityPeriodSetting",
    verb: "get",
    summary: "見積有効期限の設定を取得",
    displayName: "見積有効期限設定",
  },
  {
    id: "InvoiceDueDateDefaultSetting",
    verb: "update",
    summary: "請求書支払期日の既定値を更新",
    displayName: "請求書支払期日設定",
  },
  {
    id: "ComplaintCategory",
    verb: "list",
    summary: "クレーム区分の一覧",
    displayName: "クレーム区分一覧",
  },
  {
    id: "BundleDiscountRuleSetting",
    verb: "get",
    summary: "セット商品割引ルールの設定を取得",
    displayName: "セット商品割引設定",
  },
  {
    id: "SampleRequestLimitSetting",
    verb: "update",
    summary: "サンプル依頼の上限設定を更新",
    displayName: "サンプル依頼上限設定",
  },
];

const workflows: readonly Workflow[] = [
  { id: "Order", noun: "受注", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "Quotation", noun: "見積", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "ReturnOrder", noun: "返品", actions: ["submit", "approve"] },
];

export const sales: ServiceFixture = {
  name: "sales",
  displayName: "販売管理",
  resources,
  aggregates,
  settings,
  workflows,
};
