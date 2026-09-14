/**
 * Axis C (docs/specs/narrowing.md section 4): near-neighbours inside one
 * service. Each question names a group key from the fixture (`group` on
 * `Resource`) and has two or more defensible answers, all in the same
 * service — the worked example is inventory's "stock" group: 在庫品目 /
 * 在庫ロット / 在庫引当 / 棚卸 / 在庫調整, all roughly equal candidates for
 * 「在庫を見たい」.
 */
import type { Question } from "./types.ts";

export const AXIS_C: readonly Question[] = [
  // inventory — group "stock", "movement", "master".
  {
    id: "c01",
    axis: "C",
    text: "在庫を見たい",
    answers: [
      "listInventoryItems",
      "listInventoryLots",
      "listInventoryAllocations",
      "listInventoryStockCounts",
      "listInventoryAdjustments",
    ],
  },
  {
    id: "c02",
    axis: "C",
    text: "入出庫を一覧したい",
    answers: ["listInventoryReceivings", "listInventoryShipments"],
  },
  {
    id: "c03",
    axis: "C",
    text: "品目にまつわる基準情報を知りたい",
    answers: ["listInventoryStorageConditions", "listInventoryLotAttributes"],
  },
  {
    id: "c04",
    axis: "C",
    text: "棚卸や調整の記録を見たい",
    answers: ["listInventoryStockCounts", "listInventoryAdjustments"],
  },
  {
    id: "c05",
    axis: "C",
    text: "倉庫やロケーションの情報を知りたい",
    answers: ["listInventoryWarehouses", "listInventoryStorageLocations"],
  },
  // sales — group "order", "customer", "pricing".
  {
    id: "c06",
    axis: "C",
    text: "受注に関わる書類を確認したい",
    answers: ["listSalesQuotations", "listSalesDeliveryNotes"],
  },
  {
    id: "c07",
    axis: "C",
    text: "得意先まわりの情報を確認したい",
    answers: ["listSalesCustomers", "listSalesContacts"],
  },
  {
    id: "c08",
    axis: "C",
    text: "取引先や与信の情報を知りたい",
    answers: ["listSalesPartners", "listSalesCreditLimits"],
  },
  {
    id: "c09",
    axis: "C",
    text: "価格や割引の設定を見たい",
    answers: ["listSalesPriceLists", "listSalesDiscounts"],
  },
  {
    id: "c10",
    axis: "C",
    text: "販促施策の情報を知りたい",
    answers: ["listSalesCampaigns", "listSalesCoupons"],
  },
  // purchasing — group "order", "supplier", "approval-flow".
  {
    id: "c11",
    axis: "C",
    text: "発注に関わる書類を確認したい",
    answers: ["listPurchasingPurchaseRequests", "listPurchasingGoodsReceipts"],
  },
  {
    id: "c12",
    axis: "C",
    text: "検収や請求の状況を見たい",
    answers: ["listPurchasingGoodsReceipts", "listPurchasingInvoices"],
  },
  {
    id: "c13",
    axis: "C",
    text: "仕入先の評価や契約を知りたい",
    answers: ["listPurchasingContracts", "listPurchasingSupplierEvaluations"],
  },
  {
    id: "c14",
    axis: "C",
    text: "予算やコストセンターの情報を見たい",
    answers: ["listPurchasingBudgets", "listPurchasingCostCenters"],
  },
  {
    id: "c15",
    axis: "C",
    text: "支出限度額や決裁委任の設定を確認したい",
    answers: ["listPurchasingSpendingLimits", "listPurchasingDelegations"],
  },
  // attendance — group "time", "leave", "org".
  {
    id: "c16",
    axis: "C",
    text: "残業や欠勤の状況を確認したい",
    answers: ["listAttendanceOvertimes", "listAttendanceAbsences"],
  },
  {
    id: "c17",
    axis: "C",
    text: "打刻や遅刻早退の記録を見たい",
    answers: ["listAttendanceClockEvents", "listAttendanceLatenesss"],
  },
  {
    id: "c18",
    axis: "C",
    text: "休暇の申請状況を知りたい",
    answers: ["listAttendanceLeaveRequests", "listAttendanceLeaveBalances"],
  },
  {
    id: "c19",
    axis: "C",
    text: "特別休暇や休日の設定を確認したい",
    answers: ["listAttendanceSpecialLeaves", "listAttendanceHolidays"],
  },
  {
    id: "c20",
    axis: "C",
    text: "部署やシフトの情報を見たい",
    answers: ["listAttendanceDepartments", "listAttendanceShifts"],
  },
  // expense — group "claim", "approval-flow", "card".
  {
    id: "c21",
    axis: "C",
    text: "経費申請にまつわる書類を確認したい",
    answers: ["listExpenseReceipts", "listExpenseTravelExpenses"],
  },
  {
    id: "c22",
    axis: "C",
    text: "仮払や清算の状況を見たい",
    answers: ["listExpenseAdvances", "listExpenseSettlements"],
  },
  {
    id: "c23",
    axis: "C",
    text: "予算や費目の設定を確認したい",
    answers: ["listExpenseBudgets", "listExpenseCostCategories"],
  },
  {
    id: "c24",
    axis: "C",
    text: "経費限度額や予算の情報を知りたい",
    answers: ["listExpenseSpendingLimits", "listExpenseBudgets"],
  },
  {
    id: "c25",
    axis: "C",
    text: "法人カードやカード利用の記録を見たい",
    answers: ["listExpenseCorporateCards", "listExpenseCardTransactions"],
  },
];
