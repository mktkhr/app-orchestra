/**
 * 在庫 (inventory) — 200 operations: 30 resources × 5 verbs, 20 aggregates,
 * 20 settings, 10 workflow actions. See docs/specs/narrowing.md section 3.
 */
import { ALL_VERBS, pluralOf } from "./naming.ts";
import type { Aggregate, Resource, ServiceFixture, Setting, Workflow } from "./types.ts";

function r(id: string, noun: string, group?: string, also?: readonly string[]): Resource {
  return {
    id,
    plural: pluralOf(id),
    noun,
    verbs: ALL_VERBS,
    ...(group === undefined ? {} : { group }),
    ...(also === undefined ? {} : { also }),
  };
}

const resources: readonly Resource[] = [
  // axis C group "stock": the worked example in docs/specs/narrowing.md section 4.
  r("Item", "在庫品目", "stock", ["品番", "品名"]),
  r("Lot", "在庫ロット", "stock"),
  r("Allocation", "在庫引当", "stock"),
  r("StockCount", "棚卸", "stock"),
  r("Adjustment", "在庫調整", "stock"),
  // axis C group "movement"
  r("Warehouse", "倉庫", "movement"),
  r("StorageLocation", "保管ロケーション", "movement"),
  r("Receiving", "入庫", "movement"),
  r("Shipment", "出庫", "movement"),
  r("Transfer", "移動", "movement"),
  // axis C group "master"
  r("ItemMaster", "品目マスタ", "master"),
  r("UnitOfMeasure", "単位", "master"),
  r("StorageCondition", "保管条件", "master"),
  r("LotAttribute", "ロット属性", "master"),
  // ungrouped
  r("Supplier", "仕入先"),
  r("Category", "品目カテゴリ"),
  r("Barcode", "バーコード"),
  r("SerialNumber", "シリアル番号"),
  r("ExpiryDate", "使用期限"),
  r("ReorderPoint", "発注点"),
  r("SafetyStock", "安全在庫"),
  r("CostLayer", "原価レイヤー"),
  r("PickList", "ピッキングリスト"),
  r("PackingList", "梱包リスト"),
  r("Container", "コンテナ"),
  r("Pallet", "パレット"),
  r("QualityInspection", "品質検査"),
  r("VendorReturn", "仕入先返品"),
  r("StockAlert", "在庫アラート"),
  r("Kit", "キット構成"),
];

const aggregates: readonly Aggregate[] = [
  { id: "Items", kind: "search", noun: "在庫品目" },
  { id: "Lots", kind: "search", noun: "在庫ロット" },
  { id: "StockLevel", kind: "summarize", noun: "在庫量" },
  { id: "Adjustments", kind: "summarize", noun: "在庫調整" },
  { id: "Allocations", kind: "aggregate", noun: "在庫引当" },
  { id: "Warehouses", kind: "search", noun: "倉庫" },
  { id: "StorageLocations", kind: "search", noun: "保管ロケーション" },
  { id: "Receivings", kind: "summarize", noun: "入庫" },
  { id: "Shipments", kind: "summarize", noun: "出庫" },
  { id: "Transfers", kind: "aggregate", noun: "移動" },
  { id: "StockCounts", kind: "summarize", noun: "棚卸" },
  { id: "ExpiringItems", kind: "search", noun: "使用期限切れ間近品目" },
  { id: "OutOfStockItems", kind: "search", noun: "在庫切れ品目" },
  { id: "SafetyStockGap", kind: "aggregate", noun: "安全在庫差異" },
  { id: "CostLayers", kind: "aggregate", noun: "原価レイヤー" },
  { id: "PickLists", kind: "search", noun: "ピッキングリスト" },
  { id: "PackingLists", kind: "search", noun: "梱包リスト" },
  { id: "QualityInspections", kind: "summarize", noun: "品質検査" },
  { id: "VendorReturns", kind: "summarize", noun: "仕入先返品" },
  { id: "StockAlerts", kind: "search", noun: "在庫アラート" },
];

// axis E: settings named after transactions they do not answer for. Decoys
// are marked so a reader can see the design; the type itself carries none.
const settings: readonly Setting[] = [
  {
    id: "SafetyStockThreshold",
    verb: "get",
    summary: "安全在庫数の下限設定を取得",
    displayName: "安全在庫下限設定",
  },
  {
    id: "ReorderPointThreshold",
    verb: "update",
    summary: "発注点の閾値を更新",
    displayName: "発注点閾値設定",
  },
  {
    id: "UnitOfMeasureCategory",
    verb: "list",
    summary: "単位カテゴリの一覧",
    displayName: "単位カテゴリ一覧",
  },
  {
    id: "WarehouseCapacitySetting",
    verb: "get",
    summary: "倉庫容量設定を取得",
    displayName: "倉庫容量設定",
  },
  {
    id: "QuarantinePeriodSetting",
    verb: "update",
    summary: "検品保留期間の設定を更新",
    displayName: "検品保留期間設定",
  },
  {
    id: "StockAlertCategory",
    verb: "list",
    summary: "在庫アラート区分の一覧",
    displayName: "在庫アラート区分一覧",
  },
  {
    id: "CycleCountFrequencySetting",
    verb: "get",
    summary: "実地棚卸の頻度設定を取得",
    displayName: "棚卸頻度設定",
  },
  {
    id: "AdjustmentApprovalThreshold",
    verb: "update",
    summary: "在庫調整の承認金額しきい値を更新",
    displayName: "在庫調整承認しきい値",
  },
  {
    id: "ExpiryAlertLeadTimeSetting",
    verb: "get",
    summary: "使用期限アラートのリードタイム設定を取得",
    displayName: "使用期限アラート設定",
  },
  {
    id: "LotAttributeCategory",
    verb: "list",
    summary: "ロット属性区分の一覧",
    displayName: "ロット属性区分一覧",
  },
  {
    id: "PickingPrioritySetting",
    verb: "get",
    summary: "ピッキング優先度の設定を取得",
    displayName: "ピッキング優先度設定",
  },
  {
    id: "PackingDefaultSetting",
    verb: "update",
    summary: "梱包デフォルト設定を更新",
    displayName: "梱包デフォルト設定",
  },
  {
    id: "SupplierCategory",
    verb: "list",
    summary: "仕入先区分の一覧",
    displayName: "仕入先区分一覧",
  },
  {
    id: "BarcodeFormatSetting",
    verb: "get",
    summary: "バーコード形式の設定を取得",
    displayName: "バーコード形式設定",
  },
  {
    id: "ContainerCapacitySetting",
    verb: "update",
    summary: "コンテナ容量の設定を更新",
    displayName: "コンテナ容量設定",
  },
  {
    id: "KitComponentCategory",
    verb: "list",
    summary: "キット構成区分の一覧",
    displayName: "キット構成区分一覧",
  },
  {
    id: "CostLayerMethodSetting",
    verb: "get",
    summary: "原価レイヤー算出方式の設定を取得",
    displayName: "原価算出方式設定",
  },
  {
    id: "QualityInspectionThreshold",
    verb: "update",
    summary: "品質検査の合格基準を更新",
    displayName: "品質検査合格基準",
  },
  {
    id: "ReturnReasonCategory",
    verb: "list",
    summary: "返品理由区分の一覧",
    displayName: "返品理由区分一覧",
  },
  {
    id: "PalletCapacitySetting",
    verb: "get",
    summary: "パレット積載量の設定を取得",
    displayName: "パレット積載量設定",
  },
];

const workflows: readonly Workflow[] = [
  { id: "Adjustment", noun: "在庫調整", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "StockCount", noun: "棚卸", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "Transfer", noun: "移動", actions: ["submit", "approve"] },
];

export const inventory: ServiceFixture = {
  name: "inventory",
  displayName: "在庫管理",
  resources,
  aggregates,
  settings,
  workflows,
};
