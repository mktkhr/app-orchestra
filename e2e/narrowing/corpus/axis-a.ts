/**
 * Axis A (docs/specs/narrowing.md section 4): the verb carries no
 * selectivity. Every service lists things, so all of the selectivity has to
 * be in the noun. Each question below names a distinctive resource with a
 * single defensible answer, phrased the way a person would ask rather than
 * as `<noun>の一覧` with a suffix stapled on.
 */
import type { Question } from "./types.ts";

export const AXIS_A: readonly Question[] = [
  {
    id: "a01",
    axis: "A",
    text: "バーコードの発行状況を見せて",
    answers: ["listInventoryBarcodes"],
  },
  {
    id: "a02",
    axis: "A",
    text: "シリアル番号ってどうなってる？",
    answers: ["listInventorySerialNumbers"],
  },
  {
    id: "a03",
    axis: "A",
    text: "ピッキングリストが知りたい",
    answers: ["listInventoryPickLists"],
  },
  {
    id: "a04",
    axis: "A",
    text: "コンテナがいくつあるか教えて",
    answers: ["listInventoryContainers"],
  },
  { id: "a05", axis: "A", text: "パレットって何個ある？", answers: ["listInventoryPallets"] },
  {
    id: "a06",
    axis: "A",
    text: "キット構成を確認したい",
    answers: ["listInventoryKits"],
  },
  {
    id: "a07",
    axis: "A",
    text: "営業担当者って誰がいる？",
    answers: ["listSalesReps"],
  },
  { id: "a08", axis: "A", text: "営業エリアってどこがある？", answers: ["listSalesTerritories"] },
  {
    id: "a09",
    axis: "A",
    text: "販売予測を見せてほしい",
    answers: ["listSalesForecasts"],
  },
  { id: "a10", axis: "A", text: "ギフトカードの状況が知りたい", answers: ["listSalesGiftCards"] },
  {
    id: "a11",
    axis: "A",
    text: "サンプル依頼って今どうなってる？",
    answers: ["listSalesSampleRequests"],
  },
  { id: "a12", axis: "A", text: "セット商品を出したい", answers: ["listSalesBundles"] },
  {
    id: "a13",
    axis: "A",
    text: "入札の状況を確認したい",
    answers: ["listPurchasingBids"],
  },
  {
    id: "a14",
    axis: "A",
    text: "単価契約って今どうなってる？",
    answers: ["listPurchasingBlankets"],
  },
  {
    id: "a15",
    axis: "A",
    text: "輸入申告ってどうなってる？",
    answers: ["listPurchasingImportDeclarations"],
  },
  {
    id: "a16",
    axis: "A",
    text: "関税ってどれくらいかかってる？",
    answers: ["listPurchasingCustomsDuties"],
  },
  {
    id: "a17",
    axis: "A",
    text: "直送の案件を見せて",
    answers: ["listPurchasingDirectShips"],
  },
  {
    id: "a18",
    axis: "A",
    text: "健康診断って受けた人いる？",
    answers: ["listAttendanceHealthCheckups"],
  },
  { id: "a19", axis: "A", text: "研修受講の状況が知りたい", answers: ["listAttendanceTrainings"] },
  {
    id: "a20",
    axis: "A",
    text: "資格を持ってる人を出したい",
    answers: ["listAttendanceQualifications"],
  },
  {
    id: "a21",
    axis: "A",
    text: "苦情申立って今どうなってる？",
    answers: ["listAttendanceGrievances"],
  },
  {
    id: "a22",
    axis: "A",
    text: "法人カードって何枚ある？",
    answers: ["listExpenseCorporateCards"],
  },
  {
    id: "a23",
    axis: "A",
    text: "接待交際費ってどれくらい使ってる？",
    answers: ["listExpenseEntertainments"],
  },
  {
    id: "a24",
    axis: "A",
    text: "手当の支給状況が知りたい",
    answers: ["listExpenseAllowances"],
  },
  { id: "a25", axis: "A", text: "日当の支給状況を確認したい", answers: ["listExpensePerDiems"] },
];
