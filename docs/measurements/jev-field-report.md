# Jev フィールドレポート — 実際の使われ方と本プロジェクトへの転用可能性

Date: 2026-09-17 (JST)。TypeSafe の Jev / System One models は 2026-09-15 に
early access として公開された（**vendor/press**、[Hacker News スレッド](https://news.ycombinator.com/item?id=49717558)
の要約より、DCVC 主導 $40M シード、創業者 Diogo Almeida / Erik Gafni / Sasha
Sheng）。本調査は公開から2日後に行っており、見つかる独立事例の大半は
「発表直後の速報・自主検証」段階のものであることを先に断っておく。

調査方法: WebSearch と WebFetch。`docs.typesafe.ai` の cookbook / concepts /
blog、GitHub 上で `api.typesafe.ai` / `jev-latest` / `systemone` を使うコード、
Hacker News、Qiita/Zenn の日本語記事、独立ブログを横断的に検索した。各項目に
**vendor**（TypeSafe自身の発表・ドキュメント）/ **practitioner**（第三者が
実際に動かした記録）/ **inference**（本レポートの推論）のラベルを付ける。
URL を確認できなかった数値は「未確認」と明記し、本文の結論には使わない。

## 0. 要約（先に結論）

- 独立した「本番で使い続けている」証拠はほぼゼロ。見つかった中で最も本番に
  近いのは GitHub 上の実コード変更 [usenotra/notra PR#1106](https://github.com/usenotra/notra/pull/1106)
  （2026-09-17 マージ済み）1件のみ。他はすべて発表2日以内の検証記事・PR提案
  （未マージ含む）・issue。
- 日本語での利用実績は複数見つかったが（Qiita/Zenn）、いずれも個人の検証
  記事で、精度評価ではなくレイテンシとスキーマ準拠の確認にとどまる。
- 「1回の呼び出しに複数の決定を詰め込む」ワークロードは vendor と
  practitioner の双方が「質問を増やしてもレイテンシがほぼ変わらない」と
  報告しているが、検証対象は当プロジェクトの2問（311ms vs 220ms）より
  桁の大きい 12〜13 問のバッチであり、2問という小さいバッチでの効果は
  誰も個別には測っていない。
- 100 件を超える選択肢でのルーティング精度を報告した独立事例は見つから
  なかった。「cardinality 255」という数字は WebSearch の要約にのみ現れ、
  一次ドキュメントへの直接到達はできなかった（**未確認**）。
- confidence threshold の実運用例は2件（GitHub issue の提案、GitHub PR の
  実装）で、いずれも 0.9 をしきい値に採用しているが、両者とも閾値選定の
  根拠は説明していない — 当プロジェクトが 0.5/0.7 を根拠なく選んだのと
  同じ状態にある。

---

## 1. ベンダー自身の資料（vendor）

### 1.1 API の形

- エンドポイント: `POST https://api.typesafe.ai/v1/systemone`、デフォルト
  モデルは `jev-latest`。公式 SDK は Python と JavaScript。
  ([docs.typesafe.ai/concepts/system-one](https://docs.typesafe.ai/concepts/system-one)、閲覧 2026-09-17)
- 3つの質問形: Choice（選択肢から1つ）、Score（段階評価、正規化スケール）、
  Noul（0-1の確率で答える yes/no）。「every answer coming back with a
  calibrated confidence number」(同上)。
- state は「unstructured text or program state」1つに対し、複数の
  typed questions を同時に投げられる。「each question is evaluated in
  parallel and in isolation against the same state」
  ([docs.typesafe.ai/cookbooks/parallel_questions](https://docs.typesafe.ai/cookbooks/parallel_questions)、閲覧 2026-09-17)。
- 入力は「Text input only」現状 (同 concepts ページ)。

### 1.2 バッチ（1回の呼び出しに複数の決定）— vendor の実験例

`parallel_questions` cookbook は GDPR の Wikipedia 記事（約54,000字）を state
にして、8 Noul + 2 Choice + 3 Score = **13問を1回で投げる例**を示す。

- バッチ: 0.27秒。逐次13回呼び出し: 2.71秒。「12.2x cheaper, 10.0x faster」
  (同cookbook)。
- 5回の再実行で回答の分散が「std dev exactly 0.0」— バッチにしても質問間で
  答えがぶれない、という一貫性の主張。
- **[label: vendor]**。質問数が2問（当プロジェクトの実測: 311ms vs 220ms、
  約+41%）のような小さいバッチでの挙動は、この13問の例からは外挿できない
  — 13問という大きなバッチでの「ほぼ変わらない」効果を、2問のケースに
  そのまま適用してよい根拠にはならない。

### 1.3 レイテンシ・価格・精度（vendor blog）

[typesafe.ai/blog/introducing-system-one-models-and-jev](https://typesafe.ai/blog/introducing-system-one-models-and-jev)
（2026-09-15、**vendor**）:

- end-to-end レスポンス 70–500ms、入力 $0.042/MTok、出力無料。
- 独自の4ワークロード・711ケースのダッシュボードで Jev 67.8%、比較対象の
  最良モデル 74.1%（内訳: security incidents Jev 61.7% vs Opus 5 66.2%、
  agent-trace observability 71.6% vs 76.6%、invoice processing 61.8% vs
  79.1%、customer service 76.0% vs 78.3%）。**ベンダー自身の内部評価**で
  あり、独立追試は見つからなかった。
- Doom bot デモ（10 queries/秒、約$7/時間）、Wikiracing デモ。いずれも
  マーケティング用デモで、独立した性能追試なし。
- 「cardinality up to 255」「超える場合は score→選抜の2段構成にする」と
  いう記述が複数の二次記事の要約に現れたが、cookbook 一覧ページを直接
  読んでもこのページ自体には到達できず、**一次ソース未確認**として扱う。

### 1.4 キャリブレーション（vendor の主張）

RLCD（Reinforcement Learning for Calibrated Decisions、Brier score ベースの
報酬）で学習しており、「higher confidence means higher accuracy」と謳う
（複数の二次記事が引用、一次のトレーニング詳細ページは非公開）。
**[label: vendor、詳細は未開示]**。

---

## 2. Hacker News の反応（practitioner、懐疑的コメント中心）

[news.ycombinator.com/item?id=49717558](https://news.ycombinator.com/item?id=49717558)
（2026-09-15〜、**practitioner**、多数コメント）

- jacobgold: "can still emit a completely wrong valid value" — 型が
  正しいことと値が正しいことは別、という指摘。
- 8note: "if it puts a high confidence value on a wrong answer, thats
  still hallucinating" — confidence score があっても誤答は誤答。
- WhitneyLand: 当初のタイトル「40-400x cheaper and 20-200x faster」を
  "misleading" と批判（比較対象がフルの chain-of-thought 生成であり
  fair な比較ではない、という趣旨のコメントも複数）。
- bigglebear: 大きい選択肢空間を扱うには「hand holding here, and map
  out your problem space manually」— 選択肢設計の手間を指摘。
- porridgeraisin: "I got accepted from the waitlist and it's really
  neat" — 早期アクセスの感想のみ、定量データなし。
- **日本語・非英語利用、100件超の選択肢、1回あたり複数決定のバッチ化に
  ついての言及はスレッド中に見つからなかった。**

---

## 3. 独立した使用実績（practitioner）

### 3.1 dev.classmethod.jp（日本、Classmethod）— LLMルーティングの代替検証

[I tried replacing model routing with TypeSafe (Jev)](https://dev.classmethod.jp/en/articles/jev-for-llm-model-routing/)
Morinaga Taishi (森永大志)、2026-09-17、**practitioner**。

- 決定: 会話の難易度分類による LLM モデルルーティング（4択:
  simple/medium/complex/reasoning）。state は直近4ターンの会話要約
  （NVIDIA-NeMo Switchyard の手法を模倣）。
- 実測: median latency 0.643–0.674秒、1件あたり $0.000025–0.000027、
  各ティア10問ずつ計40問で全問正解。confidence は simple/complex/
  reasoning で1.0、medium のみ 0.57–0.67（medium だけ確信度が下がった
  ことを「LLMベースのテキスト分類ではあまり見られない利点」と好意的に
  評価）。
- 比較: 同著者の以前の検証にある Gemini 3.5 Flash（2.1秒、$0.70/session）、
  DeepSeek V4 Flash（7.2秒、$0.0004/session）。
- 継続性: NeMo Switchyard への本統合は「今回は行っていない」、
  アダプタの実装は「別記事に持ち越し」— **検証段階、採用未確定**。

### 3.2 GitHub `usenotra/notra` PR#1106 — マージ済みの実運用コード変更

[usenotra/notra PR#1106](https://github.com/usenotra/notra/pull/1106)
janburzinski、2026-09-17 マージ、**practitioner（実コード、最も本番に近い
事例）**。

- 3箇所の分類器を Jev 化、LLM フォールバック付き:
  chat autorouter（complexity / requiresTools / reasoningHeavy）、
  feedback classifier（kind / sentiment）、GEO judge（sentiment /
  position）。
- 153件の手ラベル付きテストケース、3回の実行:
  - chat router: latency 1.319s → 307ms、accuracy 81.7% → 90.0%
  - GEO judge: accuracy 52.2% → 88.0%、null sentiment バグ解消
  - feedback: accuracy 76.4% → 94.4%、タイムアウト失敗 10/72 → 0/72
- feature flag `NOTRA_JEV_CLASSIFIERS=off` で即ロールバック可能、
  タイムアウト2.5秒、confidence をログに保持。
- **これが唯一、独立第三者による「精度が上がり、マージされて残った」
  事例。** ただし1リポジトリ・1PRであり、長期運用の追跡記録はまだ無い
  （PR自体が調査当日にマージされたばかり）。

### 3.3 GitHub `NVIDIA-NeMo/Switchyard` PR#724 — 小規模スモークテスト

[NVIDIA-NeMo/Switchyard PR#724](https://github.com/NVIDIA-NeMo/Switchyard/pull/724)
pst2154、2026-09-17、**practitioner（探索段階、本番化前の評価PR）**。

- Jev (`jev-latest`) vs `gpt-5.6-sol` 生成分類器、10ケースの比較。
- Jev: mean latency 281ms、median 278ms、p95 375ms、10/10正解。
  生成分類器側のレイテンシ数値は記載なし。
- 著者自身の注記: "This gives the TypeSafe router proposal an
  evidence-backed starting point **without presenting the small smoke
  test as a production benchmark**." — 10ケースのスモークテストを
  本番ベンチマークとして扱うな、と明言。本番導入前の5段階計画
  （閾値調整・大規模ランダム化テスト・フォールバック経路検証を含む）を
  提案するのみで、**未採用**。

### 3.4 GitHub `mas-bandwidth/nova-tools` issue#896 — 複数決定点への適用提案

[mas-bandwidth/nova-tools issue#896](https://github.com/mas-bandwidth/nova-tools/issues/896)
2026-09-17、**practitioner（R&D issue、未マージ）**。

- 5種類の決定に Jev を提案: card routing（work type 6択 + complexity
  0-3のscore）、abstain reason（needs_human の noul）、note triage
  （inbox item 6択 + needs_reply noul）、CI flaky-vs-real、宇宙ゲームの
  脅威評価（シミュレーション状態からの判定）。
- card routing: 実カード20件で19/20正解、confidence≥0.9が16/20、
  レイテンシ436ms/件、入力937トークン・出力0トークン。private data
  検出は4件の secrets 関連カードで precision 0.4–0.57（低め）。
- abstain reason: 30タスク中、confidence>0.9 が9件（全て一貫）、
  0.9未満の21件は人間レビューに回す設計。
- 脅威評価: 60状態の合成データで danger score の平均絶対誤差 0.18、
  衝突検知60/60正解、maneuver決定は51/60（confidenceが下がる場面）。
  **throughput: concurrency 10 で 22 calls/秒、p50 399ms** — 見つかった
  唯一の並行実行スループット実測値。
- confidence floor は 0.9 を提案（根拠の説明はなし）。「Production
  status is not explicitly confirmed — the issue documents initial R&D
  findings」— **未採用、提案段階**。

### 3.5 GitHub `Canonry/canonry` issue#1170 — 慎重な導入提案（フェイルオープン前提）

[Canonry/canonry issue#1170](https://github.com/Canonry/canonry/issues/1170)
2026-09-17、**practitioner（提案のみ、未着手）**。

- 4決定点を対象に提案するが、いずれも数値実測はまだ無い。著者自身が
  「Treat speed/cost/calibration claims as hypotheses to measure on
  Canonry data.」と明記 — ベンダーの主張を検証すべき仮説として扱う
  姿勢。「Do not send answer text off-box by default」「fail open to
  the current `generateText` path」というガードレールを先に設計。
  **未採用**。

### 3.6 Qiita（日本語）— harupython、12問バッチの検証

[Jevはなぜ速いのか？日本語で12項目をまとめて判定させてみた](https://qiita.com/harupython/items/2728bf499ab9a6782b0b)
2026-09-17、**practitioner**。

- 日本語の記事企画（トピック・構成・目的・想定読者・投稿先候補を
  JSONのstateとして渡す）に対し、Score×8 + Choice×4 = 12問を1回で
  実行。
- 実測: 「実行画面には `96ms + 212ms` と表示されていました」— 合計
  約308ms（12問一括）。
- 投稿先の choice: Qiita 92% / Zenn 8%。技術的深さの score:
  1.97/2.0（level 2の確信度97%）。記事分類は
  technical_analysis 52% / hands_on 35% と割れ、確信度36%と低め
  （きちんと「割れている」ことが確率分布に出ている、という観察）。
- 著者の注記: 「日本語で書いた記事企画から、8つのScoreと4つの
  Choiceがまとめて返ってきた」— 日本語入力は特別な前処理なく機能した。
  ただし「0%エラー率はスキーマ準拠のみを意味し、判断精度ではない」と
  釘を刺している。

### 3.7 Zenn（日本語）— kun432、Playground/curl/SDKの一通り確認

[typesafe.ai の「Jev」を試す](https://zenn.dev/kun432/scraps/7d699847974237)
2026-09-17、**practitioner**。

- 日本語のカスタマーサポート文面（Stripe接続障害3日間）を state に、
  緊急度・部署ルーティング（technical/billing/sales）・不満度スコアを
  質問。urgency confidence 97%、technical routing confidence 94%、
  frustration score confidence 100%。
- curl・Python SDK（`from typesafe_sdk import Choice, Noul, Score,
TypeSafeClient`）の両方で日本語入力が問題なく通ることを確認。
- 懸念点: 「既存のゼロショット分類器で同程度の結果が出るのでは」という
  自問、UIの分かりにくさへの言及、Redditで「同種のアーキテクチャを
  1年前に自作しOSS化した」というコメントがあったことへの言及
  — 新規性への疑問。
- 結論を「自然言語でその場でタスク定義できる、汎用的な確率的分類・
  判断モデル」とまとめており、既存技術の組み合わせという評価に
  留めている。

### 3.8 ベンチマークハーネス（結果未公開）

[GitHub Menny1337/jev-lab](https://github.com/Menny1337/jev-lab)、
**practitioner**。1問/3問バッチ/レイテンシベンチマークのハーネスを
公開しているが、README は数値結果を意図的に載せていない:
「No example output or timing is an accuracy claim, speed guarantee or
service-level agreement.」— 計測の枠組みだけが先に公開され、追試可能な
生データはまだ無い。

---

## 4. 本プロジェクトの開いている問い（a〜f）への転用マッピング

前提の数字: 当プロジェクトの5ラウンド (`jev-picker-v1.md`〜`jev-v5.md`,
`jev-chip-order.md`) は corpus 78 (local) vs 69 (Jev) で local pick に
勝てておらず、2問バッチで 311ms（1問なら220ms）。

### (a) 1回の呼び出しに複数の決定を詰め込むワークロード

- vendor: 13問（8 Noul+2 Choice+3 Score、GDPR記事）で 0.27秒、逐次より
  10x速い。**[vendor]**
- practitioner (harupython, 3.6): 12問（8 Score+4 Choice、日本語）で
  96ms+212ms。ベンダーの「ほぼ増えない」主張と整合する独立の再現。
  **[practitioner、追試あり]**
- しかし誰も「2問」という小さいバッチを単独で測っていない。当プロジェクト
  の 311ms/220ms（+41%）という実測が、大きいバッチでは薄まる
  オーバーヘッド（コネクション確立・固定コストの割合）を拾っている
  可能性がある — **[inference]** 大きいバッチで測り直せば +41%より縮む
  見込みがあるが、これは未検証の仮説であり、当プロジェクトの corpus規模
  （78/69）を動かす根拠にはまだならない。

### (b) 並行実行下のスループット

- 見つかった唯一の実測: nova-tools issue#896、concurrency 10 で
  22 calls/秒、p50 399ms。**[practitioner、1件のみ]**。当プロジェクトの
  想定同時ユーザ数・本番トラフィックとの比較材料には乏しい（相手は
  合成データのシミュレーションゲーム状態）。
- vendor 側の rate limit（トークン/秒、リクエスト/分の具体的な数字）は
  WebSearch要約にのみ現れ、一次ドキュメントへの到達ができなかったため
  **未確認**として扱う。

### (c) 100件超の選択肢、narrowing段の置き換え

- 独立事例は見つからなかった。cardinality 255・「超えたら2段構成」
  というベンダーの主張も一次ソース未確認。当プロジェクトが検討している
  「1000件の中から up to 255 を直接 Jev に渡す」という設計を裏付ける
  独立データは**ゼロ**。
- 見つかった最大の選択肢数は nova-tools issue#896 の6択（card routing）
  であり、当プロジェクトの候補数（narrowing後20件、または直接255件）
  よりずっと小さい。

### (d) キャリブレーション（confidence閾値）の実運用

- 実運用でしきい値を使っている2件（nova-tools issue#896、Canonry
  issue#1170は未着手なので実質1件: nova-tools）とも 0.9 を採用、
  「なぜ0.9か」の説明はどちらにも無い。当プロジェクトが0.5/0.7を
  根拠なく選んだのと同型の状況。**[practitioner、閾値選定の方法論は
  誰も示していない]**
- 「0.95以上に絞ると精度93.7%まで上がった」という趣旨の記述が二次記事の
  集約に現れたが、個別の一次ページを直接確認できておらず**未確認**
  として扱い、結論には使わない。
- 当プロジェクト自身の既存データ（v5までの記録）が「confidenceは
  上がるが精度がそれに伴わない」ことを示している点は、外部のどの事例
  よりも直接的な証拠として重い — 外部に反証も補強も見つからなかった
  以上、自前のデータを優先すべき。**[inference]**

### (e) 非英語（日本語）利用

- Qiita (3.6)・Zenn (3.7) の2件が日本語入力での動作を確認しており、
  いずれも「特別な前処理なく通った」と報告している。ただし両方とも
  精度評価ではなく「動いた・速かった」の確認止まりで、日本語での
  正答率を英語と比較した記録は無い。**[practitioner、精度データなし]**
- Classmethod (3.1) も日本語記事だが、質問文自体は会話要約であり
  日本語固有の性能への言及はない。
- 「日本語だと精度が落ちる/落ちない」を裏付ける独立データは無い —
  当プロジェクトの69という数字が、英語圏では出ていない日本語固有の
  低下を含んでいる可能性は外部データからは否定も肯定もできない。

### (f) 想定していなかったこと

- **フェイルオープン設計の徹底**: usenotra/notra・Canonry の両方が
  「Jevが失敗/タイムアウト/未設定なら既存のLLM経路に自動で戻す」
  ことを前提に設計している（feature flag、2.5秒タイムアウト）。
  当プロジェクトの5ラウンドはJevを picker/gate として active に切替える
  実験であり、常時フォールバックを前提にした設計比較はしていない。
- **観測可能性としての確率分布の保持**: notra PR は「Probabilities
  retained in reasoning fields for observability」— 採用可否とは別に、
  確率分布をログに残すだけで得られる価値（`jev-chip-order.md` が
  すでに同じ発想でチップ順の再利用を検証済みだが、常時ログする運用は
  未検討）。
- **一貫性検証としての Noul**: 未詳細確認の self-consistency cookbook
  （1.2節末尾で触れた通り一次ページ未到達だが、二次記事は
  「mean SD 0.0102」という数字を報告）— 同じ質問を複数回投げて分散を
  見る、という使い方は当プロジェクトの5ラウンドにはない。

---

## 5. 価値×安さでランク付けした上位3テスト

| 順位 | アイデア                                                                                           | 価値 | 安さ | 理由                                                                                                                                   |
| ---- | -------------------------------------------------------------------------------------------------- | ---- | ---- | -------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | 2問バッチのレイテンシを大きいバッチと同じ条件で測り直す                                            | 高   | 高   | 既存の311ms/220msという1点の数字を、バッチサイズを振って再測定するだけ。新しいモデル呼び出し設計は不要                                 |
| 2    | フェイルオープン（Jevタイムアウト時にlocalへ即フォールバック）を picker に組み込んで v6 として測る | 高   | 中   | notra/Canonryが揃って採用した設計だが、v1〜v5はいずれもJevをactiveな経路として切り替える実験で、フォールバック込みの構成を測っていない |
| 3    | corpusで既存の confidence 分布から「0.9固定」しきい値を当ててみて accept/reject が動くか見る       | 中   | 高   | 新規呼び出し不要、既存jsonl（`jev-picker-v2-corpus-picks.jsonl`等）の再集計のみ                                                        |

### 5.1 テスト1: バッチサイズを振ったレイテンシ測定

- **仮説**: 311ms/220ms(+41%)という2問の実測は、13問で10x速くなるという
  vendor/harupythonの報告と矛盾するように見えるが、実際は固定オーバー
  ヘッドの比率の違いにすぎない。
- **手順**: 同じ質問セットで、質問数を1/2/4/8/13と振って `docs/specs/eval.md`
  の runner（またはそのミニ版）でJevへの生リクエストのレイテンシだけを
  測る。narrowing/fillは絡めない、純粋なJev API呼び出しのマイクロ
  ベンチマーク。
- **instrument**: 新規スクリプト（`e2e/` 配下、既存の
  `internal/adapter/planner/jev/rawcapture_test.go` の生リクエスト形式を
  流用可能）。corpus/mid/eval標準instrumentは不要、単体のcurlループで足りる。
- **コスト**: 数十リクエスト、$0.042/MTok なので実質無視できるコスト。
  30分未満。
- **結果が変えるもの**: 質問数を増やしてもオーバーヘッドの比率が下がる
  ことが確認できれば、「多数の型付き決定を1回にまとめる」ことが
  narrowing/fill/pick統合の設計オプションとして初めて数字の裏付けを
  持つ。変わらなければ、当プロジェクトの2問という粒度ではバッチ化の
  恩恵が薄いという結論を確定できる。

### 5.2 テスト2: フェイルオープン込みの v6 として measure

- **仮説**: v1〜v5がJevをactiveな経路として切り替えて測ってきたのに対し、
  「Jevが遅い/落ちたらlocalへ即フォールバック」という notra/Canonry型の
  構成なら、Jevの弱い領域（`follow-up-other-service`等）を local が
  肩代わりし、corpus全体としてlocalの78を割らない可能性がある。
- **手順**: `internal/adapter/planner/jev.Picker` にタイムアウト
  （例: 2.5秒、notraの採用値）とエラー時フォールバックを実装し、
  `make eval`（narrowing corpus 100問）と `make eval-mid`（mid corpus
  60問）の両方で通す。
- **instrument**: `docs/specs/narrowing.md`/`shortlisting.md`の100問
  corpus、`docs/specs/midsizing.md`の60問mid corpus（`make eval-mid`）。
  既存のrunnerに手を入れるだけで新規instrumentは不要。
- **コスト**: 実装は`jev.Picker`へのオプション追加1つ、計測は
  `make eval` + `make eval-mid`の通常実行（3モデル常駐が前提、
  既存の運用コストと同じ）。半日〜1日。
- **結果が変えるもの**: corpusが78に迫る/届けば、Jevを「常時picker」
  ではなく「フォールバック付きの部分置換」として採用する設計に
  倒す根拠になる。届かなければ、フォールバックの有無はJevの弱さ
  （follow-up文脈の欠落等）を補わない、という結論になり、v1〜v5の
  否定的な結論がより強く確定する。

### 5.3 テスト3: 既存jsonlでの confidence=0.9 しきい値の再集計

- **仮説**: nova-tools/Canonryが根拠なく採用した0.9という値を、
  当プロジェクトのv1/v2 corpusデータに当ててみたとき、0.5/0.7より
  「confidence上昇と精度上昇が伴わない」問題を軽減するか。
- **手順**: `docs/measurements/jev-picker-v1-corpus-picks.jsonl` /
  `jev-picker-v2-corpus-picks.jsonl` の既存 `confidence` 列を0.9で
  区切り、閾値以上/未満で正答率がどう分かれるかを再集計するだけの
  オフライン分析（`jev-chip-order.md`と同じ「モデル呼び出しなし」の
  やり方）。
- **instrument**: 既存jsonl2本のみ。新規呼び出し・新規corpusは不要。
- **コスト**: スクリプト1本、数分。実質無料。
- **結果が変えるもの**: 0.9でも精度が伴わなければ、「Jevのconfidence
  はこの用途ではどの閾値を選んでも実用的な篩いにならない」という、
  閾値選定そのものを諦める根拠が得られる。伴えば、0.5/0.7という
  当てずっぽうの閾値を0.9系に変える具体的な理由になる。

---

## 6. 率直な結論

Jevは発表から2日という段階で、独立した「使われ方」のデータはまだ薄い。
見つかったもののほとんど（HN・Qiita・Zenn・GitHub issueの大半）は
発表直後の自主検証・提案段階であり、マージされて本番コードに残った
独立事例は `usenotra/notra` PR#1106 の1件のみ。100件超の選択肢での
ルーティング実績、非英語での精度比較、閾値選定の方法論はいずれも
外部に見つからなかった。バッチ化の効果（vendor＋Qiitaの独立追試）と
フェイルオープン設計（notra・Canonryの2件が揃って採用）だけは、複数の
独立ソースが一致して報告しており、当プロジェクトが次に測る価値が
最も高い。
