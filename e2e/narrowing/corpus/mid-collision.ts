/**
 * The mid corpus's collision questions (m31-m40, `corpus/mid.ts`), split out
 * of `mid-answerable.ts` to stay under 300 lines. One question per verb on
 * 注文 (sales/purchasing Order) and on 取引先 (sales/purchasing Partner),
 * naming both services' answers — the way the shortlist corpus's axis B
 * does.
 */
import type { MidQuestion } from "./types.ts";

export const MID_COLLISION: readonly MidQuestion[] = [
  {
    id: "m31",
    axis: "B",
    expect: "answerable",
    text: "注文の状況をまとめて確認したい",
    answers: ["listSalesOrders", "listPurchasingOrders"],
  },
  {
    id: "m32",
    axis: "B",
    expect: "answerable",
    text: "注文の中身を1件見たい",
    answers: ["getSalesOrder", "getPurchasingOrder"],
  },
  {
    id: "m33",
    axis: "B",
    expect: "answerable",
    text: "注文を新しく起こしたい",
    answers: ["createSalesOrder", "createPurchasingOrder"],
  },
  {
    id: "m34",
    axis: "B",
    expect: "answerable",
    text: "注文の内容を修正したい",
    answers: ["updateSalesOrder", "updatePurchasingOrder"],
  },
  {
    id: "m35",
    axis: "B",
    expect: "answerable",
    text: "注文をキャンセルしたい",
    answers: ["deleteSalesOrder", "deletePurchasingOrder"],
  },
  {
    id: "m36",
    axis: "B",
    expect: "answerable",
    text: "取引先を一覧でチェックしたい",
    answers: ["listSalesPartners", "listPurchasingPartners"],
  },
  {
    id: "m37",
    axis: "B",
    expect: "answerable",
    text: "取引先を1件だけ見たい",
    answers: ["getSalesPartner", "getPurchasingPartner"],
  },
  {
    id: "m38",
    axis: "B",
    expect: "answerable",
    text: "取引先を新しく登録したい",
    answers: ["createSalesPartner", "createPurchasingPartner"],
  },
  {
    id: "m39",
    axis: "B",
    expect: "answerable",
    text: "取引先の情報を書き換えたい",
    answers: ["updateSalesPartner", "updatePurchasingPartner"],
  },
  {
    id: "m40",
    axis: "B",
    expect: "answerable",
    text: "取引先を削除したい",
    answers: ["deleteSalesPartner", "deletePurchasingPartner"],
  },
];
