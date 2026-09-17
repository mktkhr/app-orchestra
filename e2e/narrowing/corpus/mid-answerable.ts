/**
 * The mid corpus's per-operation answerable questions (`corpus/mid.ts`),
 * m01-m30, one per operation, in catalogue order. The ten collision
 * questions (m31-m40) live in `mid-collision.ts`.
 */
import type { MidQuestion } from "./types.ts";

export const MID_ANSWERABLE: readonly MidQuestion[] = [
  // sales / Order (受注), prefix so-
  {
    id: "m01",
    axis: "A",
    expect: "answerable",
    text: "受注の一覧が見たい",
    answers: ["listSalesOrders"],
  },
  {
    id: "m02",
    axis: "A",
    expect: "answerable",
    text: "so-0012の内容を確認したい",
    answers: ["getSalesOrder"],
  },
  {
    id: "m03",
    axis: "A",
    expect: "answerable",
    text: "受注を登録したい、得意先は青葉商事で",
    answers: ["createSalesOrder"],
  },
  {
    id: "m04",
    axis: "A",
    expect: "answerable",
    text: "so-0007の納期を来週に変更して",
    answers: ["updateSalesOrder"],
  },
  {
    id: "m05",
    axis: "A",
    expect: "answerable",
    text: "so-0031、取り消しといて",
    answers: ["deleteSalesOrder"],
  },

  // sales / Partner (取引先), prefix so-
  {
    id: "m06",
    axis: "D",
    expect: "answerable",
    text: "得意先の一覧を確認したい",
    answers: ["listSalesPartners"],
  },
  {
    id: "m07",
    axis: "A",
    expect: "answerable",
    text: "so-0058の取引先情報を見たい",
    answers: ["getSalesPartner"],
  },
  {
    id: "m08",
    axis: "D",
    expect: "answerable",
    text: "得意先を追加したい、名前は青葉商事で",
    answers: ["createSalesPartner"],
  },
  {
    id: "m09",
    axis: "A",
    expect: "answerable",
    text: "so-0064の取引先情報を修正して",
    answers: ["updateSalesPartner"],
  },
  {
    id: "m10",
    axis: "A",
    expect: "answerable",
    text: "得意先を1件削除したい",
    answers: ["deleteSalesPartner"],
  },

  // purchasing / Order (発注), prefix po-
  {
    id: "m11",
    axis: "A",
    expect: "answerable",
    text: "発注状況を一覧で見たい",
    answers: ["listPurchasingOrders"],
  },
  {
    id: "m12",
    axis: "A",
    expect: "answerable",
    text: "po-3の発注内容を教えて",
    answers: ["getPurchasingOrder"],
  },
  {
    id: "m13",
    axis: "A",
    expect: "answerable",
    text: "発注をかけたい、仕入先は山田商店で",
    answers: ["createPurchasingOrder"],
  },
  {
    id: "m14",
    axis: "A",
    expect: "answerable",
    text: "po-15の数量を直したい",
    answers: ["updatePurchasingOrder"],
  },
  {
    id: "m15",
    axis: "A",
    expect: "answerable",
    text: "po-8、キャンセルして",
    answers: ["deletePurchasingOrder"],
  },

  // purchasing / Partner (取引先), prefix po-
  {
    id: "m16",
    axis: "D",
    expect: "answerable",
    text: "仕入先一覧を確認したい",
    answers: ["listPurchasingPartners"],
  },
  {
    id: "m17",
    axis: "A",
    expect: "answerable",
    text: "po-27の仕入先情報を見たい",
    answers: ["getPurchasingPartner"],
  },
  {
    id: "m18",
    axis: "D",
    expect: "answerable",
    text: "仕入先を新規登録したい、名前は山田商店",
    answers: ["createPurchasingPartner"],
  },
  {
    id: "m19",
    axis: "A",
    expect: "answerable",
    text: "po-40の仕入先の連絡先を変えたい",
    answers: ["updatePurchasingPartner"],
  },
  {
    id: "m20",
    axis: "A",
    expect: "answerable",
    text: "仕入先を1社削除したい",
    answers: ["deletePurchasingPartner"],
  },

  // attendance / Employee (社員), prefix att-
  {
    id: "m21",
    axis: "A",
    expect: "answerable",
    text: "社員一覧が見たい",
    answers: ["listAttendanceEmployees"],
  },
  {
    id: "m22",
    axis: "A",
    expect: "answerable",
    text: "att-045の社員情報を確認したい",
    answers: ["getAttendanceEmployee"],
  },
  {
    id: "m23",
    axis: "A",
    expect: "answerable",
    text: "社員を新規登録したい、名前は鈴木一郎で",
    answers: ["createAttendanceEmployee"],
  },
  {
    id: "m24",
    axis: "A",
    expect: "answerable",
    text: "att-102の連絡先を直して",
    answers: ["updateAttendanceEmployee"],
  },
  {
    id: "m25",
    axis: "A",
    expect: "answerable",
    text: "att-013の社員情報を削除したい",
    answers: ["deleteAttendanceEmployee"],
  },

  // attendance / LeaveRequest (休暇申請), prefix att-
  {
    id: "m26",
    axis: "A",
    expect: "answerable",
    text: "休暇申請の一覧を見たい",
    answers: ["listAttendanceLeaveRequests"],
  },
  {
    id: "m27",
    axis: "A",
    expect: "answerable",
    text: "att-058の休暇申請、中身を確認したい",
    answers: ["getAttendanceLeaveRequest"],
  },
  {
    id: "m28",
    axis: "A",
    expect: "answerable",
    text: "休暇を申請したい、来週3日間で",
    answers: ["createAttendanceLeaveRequest"],
  },
  {
    id: "m29",
    axis: "A",
    expect: "answerable",
    text: "att-019の休暇申請の日程を変更したい",
    answers: ["updateAttendanceLeaveRequest"],
  },
  {
    id: "m30",
    axis: "A",
    expect: "answerable",
    text: "att-071の休暇申請を取り下げたい",
    answers: ["deleteAttendanceLeaveRequest"],
  },
];
