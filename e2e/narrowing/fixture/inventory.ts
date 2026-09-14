/**
 * 在庫 (inventory) — 200 operations: 30 resources × 5 verbs, 20 aggregates,
 * 20 settings, 10 workflow actions. See docs/specs/narrowing.md section 3.
 */
import {
  aggregateExamples,
  resourceExamples,
  settingExamples,
  workflowExamples,
} from "./examples-inventory.ts";
import { ALL_VERBS, pluralOf } from "./naming.ts";
import type {
  Aggregate,
  Resource,
  ServiceFixture,
  Setting,
  WorkflowAction,
  Workflow,
} from "./types.ts";

function r(id: string, noun: string, group?: string, also?: readonly string[]): Resource {
  const examples = resourceExamples[id];

  return {
    id,
    plural: pluralOf(id),
    noun,
    verbs: ALL_VERBS,
    ...(group === undefined ? {} : { group }),
    ...(also === undefined ? {} : { also }),
    ...(examples === undefined ? {} : { examples }),
  };
}

function a(id: string, kind: Aggregate["kind"], noun: string): Aggregate {
  const examples = aggregateExamples[id];

  return {
    id,
    kind,
    noun,
    ...(examples === undefined ? {} : { examples }),
  };
}

function s(id: string, verb: Setting["verb"], summary: string, displayName: string): Setting {
  const examples = settingExamples[id];

  return {
    id,
    verb,
    summary,
    displayName,
    ...(examples === undefined ? {} : { examples }),
  };
}

function w(id: string, noun: string, actions: readonly WorkflowAction[]): Workflow {
  const examples = workflowExamples[id];

  return {
    id,
    noun,
    actions,
    ...(examples === undefined ? {} : { examples }),
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
  a("Items", "search", "在庫品目"),
  a("Lots", "search", "在庫ロット"),
  a("StockLevel", "summarize", "在庫量"),
  a("Adjustments", "summarize", "在庫調整"),
  a("Allocations", "aggregate", "在庫引当"),
  a("Warehouses", "search", "倉庫"),
  a("StorageLocations", "search", "保管ロケーション"),
  a("Receivings", "summarize", "入庫"),
  a("Shipments", "summarize", "出庫"),
  a("Transfers", "aggregate", "移動"),
  a("StockCounts", "summarize", "棚卸"),
  a("ExpiringItems", "search", "使用期限切れ間近品目"),
  a("OutOfStockItems", "search", "在庫切れ品目"),
  a("SafetyStockGap", "aggregate", "安全在庫差異"),
  a("CostLayers", "aggregate", "原価レイヤー"),
  a("PickLists", "search", "ピッキングリスト"),
  a("PackingLists", "search", "梱包リスト"),
  a("QualityInspections", "summarize", "品質検査"),
  a("VendorReturns", "summarize", "仕入先返品"),
  a("StockAlerts", "search", "在庫アラート"),
];

// axis E: settings named after transactions they do not answer for. Decoys
// are marked so a reader can see the design; the type itself carries none.
const settings: readonly Setting[] = [
  s("SafetyStockThreshold", "get", "安全在庫数の下限設定を取得", "安全在庫下限設定"),
  s("ReorderPointThreshold", "update", "発注点の閾値を更新", "発注点閾値設定"),
  s("UnitOfMeasureCategory", "list", "単位カテゴリの一覧", "単位カテゴリ一覧"),
  s("WarehouseCapacitySetting", "get", "倉庫容量設定を取得", "倉庫容量設定"),
  s("QuarantinePeriodSetting", "update", "検品保留期間の設定を更新", "検品保留期間設定"),
  s("StockAlertCategory", "list", "在庫アラート区分の一覧", "在庫アラート区分一覧"),
  s("CycleCountFrequencySetting", "get", "実地棚卸の頻度設定を取得", "棚卸頻度設定"),
  s(
    "AdjustmentApprovalThreshold",
    "update",
    "在庫調整の承認金額しきい値を更新",
    "在庫調整承認しきい値",
  ),
  s(
    "ExpiryAlertLeadTimeSetting",
    "get",
    "使用期限アラートのリードタイム設定を取得",
    "使用期限アラート設定",
  ),
  s("LotAttributeCategory", "list", "ロット属性区分の一覧", "ロット属性区分一覧"),
  s("PickingPrioritySetting", "get", "ピッキング優先度の設定を取得", "ピッキング優先度設定"),
  s("PackingDefaultSetting", "update", "梱包デフォルト設定を更新", "梱包デフォルト設定"),
  s("SupplierCategory", "list", "仕入先区分の一覧", "仕入先区分一覧"),
  s("BarcodeFormatSetting", "get", "バーコード形式の設定を取得", "バーコード形式設定"),
  s("ContainerCapacitySetting", "update", "コンテナ容量の設定を更新", "コンテナ容量設定"),
  s("KitComponentCategory", "list", "キット構成区分の一覧", "キット構成区分一覧"),
  s("CostLayerMethodSetting", "get", "原価レイヤー算出方式の設定を取得", "原価算出方式設定"),
  s("QualityInspectionThreshold", "update", "品質検査の合格基準を更新", "品質検査合格基準"),
  s("ReturnReasonCategory", "list", "返品理由区分の一覧", "返品理由区分一覧"),
  s("PalletCapacitySetting", "get", "パレット積載量の設定を取得", "パレット積載量設定"),
];

const workflows: readonly Workflow[] = [
  w("Adjustment", "在庫調整", ["submit", "approve", "reject", "withdraw"]),
  w("StockCount", "棚卸", ["submit", "approve", "reject", "withdraw"]),
  w("Transfer", "移動", ["submit", "approve"]),
];

export const inventory: ServiceFixture = {
  name: "inventory",
  displayName: "在庫管理",
  resources,
  aggregates,
  settings,
  workflows,
};
