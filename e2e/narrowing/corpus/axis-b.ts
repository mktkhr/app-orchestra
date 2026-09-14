/**
 * Axis B (docs/specs/narrowing.md section 4): the same resource name in more
 * than one service. `docs/specs/narrowing.md` section 3 names five shared
 * keys — order, line, approval, employee, partner — whose CRUD operations
 * collapse their `displayName` to a shared Japanese word
 * (`e2e/narrowing/fixture/openapi.ts`'s `SHARED_DISPLAY`). Each question
 * below names that shared word with a different verb, on purpose: a
 * narrowing that returns only one service's operation has removed the
 * platform's ability to ask which one was meant.
 */
import type { Question } from "./types.ts";

export const AXIS_B: readonly Question[] = [
  // shared key "order" (sales/purchasing): 受注/発注 both collapse to 注文.
  {
    id: "b01",
    axis: "B",
    text: "注文を一覧したい",
    answers: ["listSalesOrders", "listPurchasingOrders"],
  },
  {
    id: "b02",
    axis: "B",
    text: "注文を1件確認したい",
    answers: ["getSalesOrder", "getPurchasingOrder"],
  },
  {
    id: "b03",
    axis: "B",
    text: "注文を新規作成したい",
    answers: ["createSalesOrder", "createPurchasingOrder"],
  },
  {
    id: "b04",
    axis: "B",
    text: "注文の内容を更新したい",
    answers: ["updateSalesOrder", "updatePurchasingOrder"],
  },
  {
    id: "b05",
    axis: "B",
    text: "注文を削除したい",
    answers: ["deleteSalesOrder", "deletePurchasingOrder"],
  },
  // shared key "line" (sales/purchasing/expense): 受注明細/発注明細/経費明細 collapse to 明細.
  {
    id: "b06",
    axis: "B",
    text: "明細を一覧で見たい",
    answers: ["listSalesOrderLines", "listPurchasingOrderLines", "listExpenseLines"],
  },
  {
    id: "b07",
    axis: "B",
    text: "明細を1件確認したい",
    answers: ["getSalesOrderLine", "getPurchasingOrderLine", "getExpenseLine"],
  },
  {
    id: "b08",
    axis: "B",
    text: "明細を追加したい",
    answers: ["createSalesOrderLine", "createPurchasingOrderLine", "createExpenseLine"],
  },
  {
    id: "b09",
    axis: "B",
    text: "明細を修正したい",
    answers: ["updateSalesOrderLine", "updatePurchasingOrderLine", "updateExpenseLine"],
  },
  {
    id: "b10",
    axis: "B",
    text: "明細を消したい",
    answers: ["deleteSalesOrderLine", "deletePurchasingOrderLine", "deleteExpenseLine"],
  },
  // shared key "approval" (purchasing/attendance/expense): 発注承認/勤怠承認/経費承認 collapse to 承認.
  {
    id: "b11",
    axis: "B",
    text: "承認状況を一覧したい",
    answers: ["listPurchasingApprovals", "listAttendanceApprovals", "listExpenseApprovals"],
  },
  {
    id: "b12",
    axis: "B",
    text: "承認を1件見たい",
    answers: ["getPurchasingApproval", "getAttendanceApproval", "getExpenseApproval"],
  },
  {
    id: "b13",
    axis: "B",
    text: "承認を新規登録したい",
    answers: ["createPurchasingApproval", "createAttendanceApproval", "createExpenseApproval"],
  },
  {
    id: "b14",
    axis: "B",
    text: "承認内容を更新したい",
    answers: ["updatePurchasingApproval", "updateAttendanceApproval", "updateExpenseApproval"],
  },
  {
    id: "b15",
    axis: "B",
    text: "承認を取り消したい",
    answers: ["deletePurchasingApproval", "deleteAttendanceApproval", "deleteExpenseApproval"],
  },
  // shared key "employee" (attendance/expense): 社員.
  {
    id: "b16",
    axis: "B",
    text: "社員を一覧したい",
    answers: ["listAttendanceEmployees", "listExpenseEmployees"],
  },
  {
    id: "b17",
    axis: "B",
    text: "社員情報を1件見たい",
    answers: ["getAttendanceEmployee", "getExpenseEmployee"],
  },
  {
    id: "b18",
    axis: "B",
    text: "社員を新規登録したい",
    answers: ["createAttendanceEmployee", "createExpenseEmployee"],
  },
  {
    id: "b19",
    axis: "B",
    text: "社員情報を更新したい",
    answers: ["updateAttendanceEmployee", "updateExpenseEmployee"],
  },
  {
    id: "b20",
    axis: "B",
    text: "社員を削除したい",
    answers: ["deleteAttendanceEmployee", "deleteExpenseEmployee"],
  },
  // shared key "partner" (sales/purchasing): 取引先.
  {
    id: "b21",
    axis: "B",
    text: "取引先を一覧したい",
    answers: ["listSalesPartners", "listPurchasingPartners"],
  },
  {
    id: "b22",
    axis: "B",
    text: "取引先を1件確認したい",
    answers: ["getSalesPartner", "getPurchasingPartner"],
  },
  {
    id: "b23",
    axis: "B",
    text: "取引先を新規登録したい",
    answers: ["createSalesPartner", "createPurchasingPartner"],
  },
  {
    id: "b24",
    axis: "B",
    text: "取引先情報を更新したい",
    answers: ["updateSalesPartner", "updatePurchasingPartner"],
  },
  {
    id: "b25",
    axis: "B",
    text: "取引先を削除したい",
    answers: ["deleteSalesPartner", "deletePurchasingPartner"],
  },
];
