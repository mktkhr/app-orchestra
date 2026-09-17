/**
 * The mid corpus's impossible half (`corpus/mid.ts`, split out to stay under
 * 300 lines). m41-m45: the verb is not there. m46-m50: the resource is not
 * there (dropped from the subset, or belongs to a service outside it). m51-
 * m55: outside the domain entirely. m56-m60: capability questions, two
 * general and three scoped (`capability: true`).
 */
import type { MidQuestion } from "./types.ts";

export const MID_IMPOSSIBLE: readonly MidQuestion[] = [
  // --- verb not there (5) ---
  {
    id: "m41",
    axis: "A",
    expect: "impossible",
    text: "受注を集計したい",
    answers: [],
  },
  {
    id: "m42",
    axis: "A",
    expect: "impossible",
    text: "取引先を承認したい",
    answers: [],
  },
  {
    id: "m43",
    axis: "A",
    expect: "impossible",
    text: "社員情報を一括削除したい",
    answers: [],
  },
  {
    id: "m44",
    axis: "A",
    expect: "impossible",
    text: "休暇申請書を印刷したい",
    answers: [],
  },
  {
    id: "m45",
    axis: "A",
    expect: "impossible",
    text: "発注を一括承認したい",
    answers: [],
  },

  // --- resource not there (5) ---
  {
    id: "m46",
    axis: "A",
    expect: "impossible",
    text: "経費の明細を見たい",
    answers: [],
  },
  {
    id: "m47",
    axis: "A",
    expect: "impossible",
    text: "在庫の数を確認したい",
    answers: [],
  },
  {
    id: "m48",
    axis: "A",
    expect: "impossible",
    text: "倉庫の一覧が知りたい",
    answers: [],
  },
  {
    id: "m49",
    axis: "A",
    expect: "impossible",
    text: "請求書を発行したい",
    answers: [],
  },
  {
    id: "m50",
    axis: "A",
    expect: "impossible",
    text: "見積の内容を確認したい",
    answers: [],
  },

  // --- out of domain (5) ---
  {
    id: "m51",
    axis: "A",
    expect: "impossible",
    text: "今日の天気を教えて",
    answers: [],
  },
  {
    id: "m52",
    axis: "A",
    expect: "impossible",
    text: "今日って何曜日だっけ",
    answers: [],
  },
  {
    id: "m53",
    axis: "A",
    expect: "impossible",
    text: "おはようございます",
    answers: [],
  },
  {
    id: "m54",
    axis: "A",
    expect: "impossible",
    text: "123足す456はいくつ？",
    answers: [],
  },
  {
    id: "m55",
    axis: "A",
    expect: "impossible",
    text: "これを英語に翻訳して",
    answers: [],
  },

  // --- capability (5) ---
  {
    id: "m56",
    axis: "A",
    expect: "impossible",
    capability: true,
    text: "何ができるの？",
    answers: [],
  },
  {
    id: "m57",
    axis: "A",
    expect: "impossible",
    capability: true,
    text: "使える操作は？",
    answers: [],
  },
  {
    id: "m58",
    axis: "A",
    expect: "impossible",
    capability: true,
    text: "受注で何ができる？",
    answers: [],
  },
  {
    id: "m59",
    axis: "A",
    expect: "impossible",
    capability: true,
    text: "発注について何ができる？",
    answers: [],
  },
  {
    id: "m60",
    axis: "A",
    expect: "impossible",
    capability: true,
    text: "勤怠で何ができる？",
    answers: [],
  },
];
