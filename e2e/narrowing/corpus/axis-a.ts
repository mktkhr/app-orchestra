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
    // "知りたい" does not say "all of them" — a filtered search reads it as
    // easily as a plain list.
    text: "ピッキングリストが知りたい",
    answers: ["listInventoryPickLists", "searchInventoryPickLists"],
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
    // Enumerating "who is there" is what a filtered search of the same
    // resource also answers.
    text: "営業担当者って誰がいる？",
    answers: ["listSalesReps", "searchSalesReps"],
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
  {
    id: "a12",
    axis: "A",
    // "出す" is ordinary Japanese for releasing a new bundle product
    // (create), not only for bringing up the existing list.
    text: "セット商品を出したい",
    answers: ["listSalesBundles", "createSalesBundle"],
  },
  {
    id: "a13",
    axis: "A",
    // "状況を確認したい" does not distinguish "all of them" from "the
    // matching ones" — search answers it as well as list.
    text: "入札の状況を確認したい",
    answers: ["listPurchasingBids", "searchPurchasingBids"],
  },
  {
    id: "a14",
    axis: "A",
    // "今どうなってる" is a status question; a summary over blanket
    // contracts answers it as well as the raw list.
    text: "単価契約って今どうなってる？",
    answers: ["listPurchasingBlankets", "summarizePurchasingBlankets"],
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
    // "どれくらい" asks for a figure, which the aggregate over the same
    // object answers directly.
    text: "関税ってどれくらいかかってる？",
    answers: ["listPurchasingCustomsDuties", "aggregatePurchasingCustomsDuties"],
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
    // "受けた人いる？" asks whether records matching a condition exist — a
    // search reads that as naturally as a plain list.
    text: "健康診断って受けた人いる？",
    answers: ["listAttendanceHealthCheckups", "searchAttendanceHealthCheckups"],
  },
  {
    id: "a19",
    axis: "A",
    // "状況" is a status question; the summary answers it as well as the
    // raw list.
    text: "研修受講の状況が知りたい",
    answers: ["listAttendanceTrainings", "summarizeAttendanceTrainings"],
  },
  {
    id: "a20",
    axis: "A",
    // "資格を持ってる人" names a filter condition outright, which is what a
    // search operation is for.
    text: "資格を持ってる人を出したい",
    answers: ["listAttendanceQualifications", "searchAttendanceQualifications"],
  },
  {
    id: "a21",
    axis: "A",
    // "今どうなってる" is a status question; search answers it as well as list.
    text: "苦情申立って今どうなってる？",
    answers: ["listAttendanceGrievances", "searchAttendanceGrievances"],
  },
  {
    id: "a22",
    axis: "A",
    // A count is unaffected by whether the operation is called "list" or
    // "search" with no filter.
    text: "法人カードって何枚ある？",
    answers: ["listExpenseCorporateCards", "searchExpenseCorporateCards"],
  },
  {
    id: "a23",
    axis: "A",
    // "どれくらい" asks for a figure, which the summary over the same
    // object answers directly.
    text: "接待交際費ってどれくらい使ってる？",
    answers: ["listExpenseEntertainments", "summarizeExpenseEntertainments"],
  },
  {
    id: "a24",
    axis: "A",
    // "支給状況" asks how much has been paid out, which the aggregate over
    // the same object answers directly.
    text: "手当の支給状況が知りたい",
    answers: ["listExpenseAllowances", "aggregateExpenseAllowances"],
  },
  {
    id: "a25",
    axis: "A",
    text: "日当の支給状況を確認したい",
    answers: ["listExpensePerDiems", "aggregateExpensePerDiems"],
  },
];
