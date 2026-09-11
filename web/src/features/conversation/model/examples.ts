/**
 * The three examples shown before the first question (AC-F-104), one per
 * shape the slice can answer: a list, a create, and a status the enum does
 * not have.
 *
 * The button shows the question itself rather than a label for it. An
 * example question is there to teach the shape of a question worth asking,
 * which a label like "list" does not do: a person who has only seen
 * "一覧を見る" still does not know they may write a sentence.
 *
 * `shape` names what each one demonstrates, for the accessible description
 * and for tests; it is never the button's own text.
 */
export const EXAMPLE_QUESTIONS = [
  { shape: "一覧", query: "在庫の一覧を見せて" },
  { shape: "登録", query: "在庫を登録して。名前はテスト品、数量は5、引当済で" },
  { shape: "あいまいな状態", query: "破損した在庫はある？" },
] as const;
