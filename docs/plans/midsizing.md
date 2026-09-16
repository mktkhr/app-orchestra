# Midsizing — implementation plan

> **For the implementer:** Tasks 1 and 2 touch disjoint files and can run in
> parallel; Task 3 follows both; Task 4 is the measurement and is run by the
> session owner. Each task ends green and is committed on its own. Steps use
> `- [ ]`.

**Goal:** `make eval-mid` boots the fixture server on a thirty-operation,
three-service subset whose ids carry a `pattern`, asks sixty questions (forty
answerable, twenty impossible) through the running platform, and prints five
numbers: correct@1, false refusal, refused, forced, fabricated - plus latency.

**Architecture:** the subset is a table (`e2e/narrowing/fixture/mid.ts`)
naming three services, two resources each, and one id prefix per service;
`toOpenAPI` emits `pattern` on id parameters when a fixture declares a
prefix; `serve.ts`'s `start` takes the services to serve. The questions are a
second corpus file with an `expect` field. The shortlist runner gains
`--corpus mid`; `score.ts` gains the mid scorer; `print-report.ts` prints the
mid section. Nothing in the platform changes.

**Spec:** `docs/specs/midsizing.md`. Acceptance criteria: its section 7.

## Global constraints

`docs/plans/shortlisting.md`'s Global constraints hold. Here in particular:

- **The full fixture is unchanged.** `catalogOf(5)` and the thousand-operation
  OpenAPI documents are byte-identical before and after (a test compares the
  generated document of one full service against a snapshot taken at the
  start of Task 1, or asserts no `pattern` appears anywhere in it).
- **`make check` calls no model.** Fixture, corpus and scorer tests are pure.
  Verify `docker logs llama-swap 2>&1 | grep -c 'POST /v1/'` before and after.
- **The questions are written blind** (spec section 4): the writer reads the
  resource nouns and verbs, not the summaries or the written examples, and
  says so in the file's header comment.
- TS files under 300 lines (split `run.ts` / `score.ts` helpers as the
  staging work did); `vite-plus/test`; no `if`/`continue` in test bodies;
  one-argument `expect`; `git commit -- <explicit paths>`; subjects ≤ 72
  characters; the `Makefile` line needs `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`.

---

### Task 1: the mid fixture - subset, id patterns, server

**Files:**

- Create: `e2e/narrowing/fixture/mid.ts` (+ `mid.test.ts`)
- Modify: `e2e/narrowing/fixture/types.ts` (`ServiceFixture.idPrefix?: string`),
  `e2e/narrowing/fixture/openapi.ts` (emit `pattern`), `e2e/narrowing/serve.ts`
  (`start(port, served = services())`), `e2e/narrowing/fixture/index.ts`
  (export `midServices`, `midCatalog`)

**Produces:**

```ts
// fixture/mid.ts
export const MID_RESOURCES = {
  sales: ["Order", "Partner"],
  purchasing: ["Order", "Partner"],
  attendance: ["Employee", "LeaveRequest"],
} as const;
export const MID_ID_PREFIX = { sales: "so-", purchasing: "po-", attendance: "att-" } as const;
/** The three services cut to MID_RESOURCES, aggregates/settings/workflows dropped, idPrefix set. */
export function midServices(): readonly ServiceFixture[];
/** The thirty operations, in the same order operationsOf yields them. */
export function midCatalog(): readonly FixtureOperation[];
```

`toOpenAPI`: when `fixture.idPrefix` is set, every `id` path parameter (get,
update, delete) gets `pattern: "^<prefix>[0-9]+$"`; otherwise the document
is exactly what it is today. The resource ids above are the fixture's
existing ones - verify each exists in `sales.ts` / `purchasing.ts` /
`attendance.ts` (they all declare `ALL_VERBS`) and that the resulting
operation ids are e.g. `listSalesOrders`, `getPurchasingPartner`,
`deleteAttendanceLeaveRequest`; if a name differs, follow the fixture and
note it in the report.

- [ ] **Step 1: failing tests** (`mid.test.ts`): `midCatalog()` has exactly
      30 operations over 3 services, 10 each, five verbs per resource; every
      operation keeps two `x-orchestra-examples`; every get/update/delete
      declares `pattern` matching `MID_ID_PREFIX` for its service; the full
      `services()` documents contain no `pattern` (AC-M-102); `start(port,
midServices())` serves `/sales/openapi.json` with 10 exposed
      operations and answers `GET /sales/api/sales/orders` with a
      schema-valid body (read `serve.test.ts` for the pattern).
- [ ] **Step 2: run, fail.**
- [ ] **Step 3: implement.**
- [ ] **Step 4: green; `make check`; count unchanged.**
- [ ] **Step 5: commit** `feat(fixture): mid subset with id patterns`.

---

### Task 2: the sixty questions

**Files:**

- Create: `e2e/narrowing/corpus/mid.ts` (+ `mid.test.ts`)
- Modify: `e2e/narrowing/corpus/types.ts` (`MidQuestion`)

**Produces:**

```ts
export interface MidQuestion extends Question {
  readonly expect: "answerable" | "impossible";
  readonly capability?: boolean; // impossible only: list_capabilities is the refusal
}
export function midQuestions(): readonly MidQuestion[];
```

Ids `m01`–`m60`, `axis` reused from the corpus for the answerable half
(A–E by the same definitions), `"B"` for impossible ones is wrong - give the
impossible half axis `"A"` and document that axis is informational here.

Answerable, 40 - written blind: at least one per operation (30), plus ten
more on the collisions (注文 / 取引先 / 社員, with two answers where both are
defensible). At least 13 carry an argument: an id in the service's prefix
(`so-0012の内容`, `po-3の発注を取り消したい`), an enum label where the resource
declares one (read the fixture's schemas - if none of the six resources has
an enum, say so and use ids and names only), a create with a name and a
number (`取引先を追加、名前は青葉商事`).

Impossible, 20: 5 verb-not-there (集計 / 承認 / 一括削除 / 印刷 on the six
resources), 5 resource-not-there (経費, 在庫, 倉庫, 請求書, 見積 - resources of
services or of the same services outside the subset), 5 out of domain
(天気, 曜日, 挨拶, 計算, 翻訳), 5 capability (「何ができるの？」, 「使える操作は？」,
「受注で何ができる？」, 「発注について何ができる？」, 「勤怠で何ができる？」 -
`capability: true`).

- [ ] **Step 1: failing tests** (`corpus/mid.test.ts`, AC-M-103): 60
      questions, 40 / 20, unique ids `m01..m60`, every answerable question's
      `answers` non-empty and every id in `midCatalog()`, every operation of
      the thirty named by at least one question, every impossible question's
      `answers` empty, exactly five with `capability`, no question text
      duplicated, no answerable question text equal to an operation's summary
      (blindness, the corpus's own test).
- [ ] **Step 2: run, fail.**
- [ ] **Step 3: write the questions.**
- [ ] **Step 4: green; `make check`.**
- [ ] **Step 5: commit** `feat(corpus): sixty mid questions, forty answerable`.

---

### Task 3: the runner, the scorer, the report, the target

**Files:**

- Modify: `e2e/shortlist/flags.ts` (`corpusArg(): "mid" | undefined`),
  `boot.ts` (`fixture?: "full" | "mid"` → `startFixture(port, midServices())`),
  `run.ts` (`--corpus mid`: questions from `midQuestions()`, output
  `mid-<variant>.jsonl`, the `QuestionResult` row gains `expect` and
  `capability` and the form's `initial` keys+string values so the fabricated
  count can be computed from the jsonl - extend `plan-request.ts`'s parsed
  response to carry `initial`), `score.ts` (`scoreMid(rows)` → the five
  numbers + latency + misses), `report.ts` / `print-report.ts` (the mid
  section of spec section 5), `Makefile` (`eval-mid`)
- Test: `score.test.ts` (AC-M-104)

`scoreMid` rules: answerable row correct@1 iff `kind ∈ {result, form}` and
`operationId ∈ answers`; false refusal iff `kind == none` or `operationId ==
list_capabilities`; impossible row refused iff `kind == none`, or
`operationId == list_capabilities` (any impossible question - a capability
question's `list_capabilities` is right, a non-capability one's is still a
refusal); forced iff `kind ∈ {result, form}` with a catalogue operation;
fabricated: over rows with `kind == form`, a row counts when any `initial`
string value (not enum, not date-shaped `\d{4}-\d{2}-\d{2}`) is absent from
the question text (case-insensitive for ASCII).

- [ ] **Step 1: failing tests** for `scoreMid` on hand-made rows covering
      every rule above; `flags.ts` `--corpus` parsing (`mid` only, anything
      else a usage error).
- [ ] **Step 2: run, fail.**
- [ ] **Step 3: implement.** `make eval-mid` = `cd e2e && node shortlist/run.ts
--corpus mid && node shortlist/print-report.ts` (the report prints the
      mid section when a `mid-*.jsonl` exists). `--corpus mid` composes with
      `--stages` / `--thinking` / `--wording` (variant suffix as today).
- [ ] **Step 4: green; `make check`; count unchanged.**
- [ ] **Step 5: commit** (a) `feat(shortlist): --corpus mid, the mid scorer
and report`, (b) `feat(make): eval-mid` with
      `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`.

---

### Task 4: measure and record (session owner)

- [ ] `make eval-mid` (defaults: two stages, v6, thinking off, narrowing on).
      Copy jsonl + report to the scratchpad; never delete `e2e/shortlist/out/`.
- [ ] Read every miss (answerable misses, forced impossibles, fabricated
      forms) row by row; widen an answer key only where the review says the
      question was ambiguous, then re-run.
- [ ] Record in `DECISIONS.md` beside 77 / 79 and 15 / 16 (AC-M-105); note
      which way the impossible half leaned (spec section 8); `STATE.md`,
      `TODO.md`.
