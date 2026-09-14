# Narrowing a catalogue nobody can send whole — implementation plan

> **For the implementer:** one task at a time. Each task ends green and is
> committed on its own. Steps use `- [ ]` for tracking.

**Goal:** a fixture catalogue of 1000 operations, a corpus of 100 Japanese
questions that is hard in five named ways, a recall@K measurement that runs in
seconds without an LLM, and a lexical baseline measured against all of it.

**Architecture:** none of this is a service. The fixture is a definition table
in TypeScript under `e2e/narrowing/`; contracts are synthesised from it in
memory and served over HTTP only when something needs to read them the way the
platform does. The measurement reads the same table directly, so a mechanism
can be changed and re-scored in a loop.

**Spec:** `docs/specs/narrowing.md`. Acceptance criteria: its section 9.

## Global constraints

`docs/plans/proposing.md`'s Global constraints section still holds. The ones
this subproject is most likely to meet:

- **The unit tests join `make check`; the measurement does not.** The tests
  that assert the fixture's contract rules and the corpus's axes are cheap,
  need nothing running and call no LLM — they are how AC-T-103 and AC-T-105
  hold, so they run in the gate. They get there by one line in
  `e2e/vite.config.ts` (Task 1, Step 0), not by a new target. The **measurement
  run** is a deliberate target like `make eval` and is added to no dependency
  list.
- `e2e/` **is** in the web workspace, so `make check` still lints and
  typechecks every file written here (`web-lint`, `web-fmt-check`). New code
  must pass oxlint type-aware rules and `tsc` with no suppressions.
- **No suppressions of any kind** (AGENTS.md rule 2) — `@ts-ignore`,
  `oxlint-disable`, `as any` and every relative.
- 1000 lines a file (`make guard-filelen`). Five service definitions of ~200
  lines each, one file per service.
- `make fmt` before reporting; **run `make check`, not `make -k check`**.
- The `Makefile` is a protected path: a change to it needs
  `ORCHESTRA_ALLOW_HARNESS_CHANGE=1` on the commit, and only Task 4 touches it.
- `make check` must never call a real LLM. Nothing in this subproject calls one
  at all. Verify with
  `docker logs llama-swap 2>&1 | grep -c 'POST /v1/chat/completions'` before
  and after.
- Commit with the pathspec form — `git commit -- <paths>` — never
  `git add -A`. Other sessions have staged work in this repository.

## Naming, fixed once

- Service names: `inventory`, `sales`, `purchasing`, `attendance`, `expense`.
  These collide with the two real services by name. That is deliberate — the
  fixture **replaces** the real estate in `ORCHESTRA_SERVICES` when measuring;
  the two are never configured together.
- `operationId` is `<verb><Service><Resource>`: `listSalesOrders`,
  `getInventoryLot`, `updateExpenseClaim`. The service name is inside the id,
  so ids never collide across services (`make guard-operation-ids`' rule) —
  **and the collisions the corpus needs live in the summaries and display
  names instead**, which is what section 3 of the spec says real systems do.
- Paths: `/api/<service>/<kebab-plural>` and `/api/<service>/<kebab-plural>/{id}`.

---

### Task 1: the definition table and the contracts it generates

**Files:**

- Create: `e2e/narrowing/fixture/types.ts`
- Create: `e2e/narrowing/fixture/inventory.ts`, `sales.ts`, `purchasing.ts`,
  `attendance.ts`, `expense.ts`
- Create: `e2e/narrowing/fixture/index.ts`
- Create: `e2e/narrowing/fixture/openapi.ts`
- Test: `e2e/narrowing/fixture/openapi.test.ts`,
  `e2e/narrowing/fixture/shape.test.ts`

**Consumes:** nothing.

**Produces:** `services()` returning the five `ServiceFixture` values,
`catalogOf(size)` returning the first N services, and `toOpenAPI(fixture)`
returning one OpenAPI 3.0.3 document object.

- [ ] **Step 0: let the tests run**

Add `narrowing/**/*.test.ts` to `e2e/vite.config.ts`'s `test.include`, beside
`src/**/*.test.ts`. That is the whole wiring: `make check` already runs
`acceptance-e2e` over that config. Say in a comment that these, unlike the
`src/` tests, need nothing built and nothing running.

`e2e/vite.config.ts` is **not** a protected path (only the root
`vite.config.ts` is). If `harness/guard/protected-paths.sh` disagrees at commit
time, stop and report it rather than setting the override.

- [ ] **Step 1: write the types**

```ts
// e2e/narrowing/fixture/types.ts
export type Verb = "list" | "get" | "create" | "update" | "delete";

/** One resource a service owns: five operations at most, one noun. */
export interface Resource {
  /** Singular, PascalCase, unique inside a service: "SalesOrder". */
  readonly id: string;
  /** Plural form used by list: "SalesOrders". */
  readonly plural: string;
  /** The Japanese noun every summary for this resource is built from. */
  readonly noun: string;
  readonly verbs: readonly Verb[];
  /**
   * Axis C. Resources sharing a group key are near-neighbours inside one
   * service: 在庫品目 / 在庫ロット / 在庫引当 / 棚卸 / 在庫調整.
   */
  readonly group?: string;
  /**
   * Axis B. Resources sharing a key across services carry the same Japanese
   * noun in two or more places: "order" is 受注 in sales and 発注 in
   * purchasing, and both summarise as 注文.
   */
  readonly shared?: string;
  /** Extra vocabulary the description carries, for realism. */
  readonly also?: readonly string[];
}

/** Axis E lives here: settings named after the transactions they configure. */
export interface Setting {
  readonly id: string;
  readonly verb: "get" | "update" | "list";
  /** Written by hand, because the decoy is the whole point. */
  readonly summary: string;
  readonly displayName: string;
}

export interface Aggregate {
  readonly id: string;
  readonly kind: "search" | "summarize" | "aggregate";
  readonly noun: string;
}

export interface Workflow {
  readonly id: string;
  readonly noun: string;
  readonly actions: readonly ("submit" | "approve" | "reject" | "withdraw")[];
}

export interface ServiceFixture {
  readonly name: string;
  readonly displayName: string;
  readonly resources: readonly Resource[];
  readonly aggregates: readonly Aggregate[];
  readonly settings: readonly Setting[];
  readonly workflows: readonly Workflow[];
}
```

- [ ] **Step 2: write the failing shape test**

```ts
// e2e/narrowing/fixture/shape.test.ts
import { expect, test } from "vitest";
import { catalogOf, operationCount, services } from "./index.ts";

test("every service declares exactly 200 operations", () => {
  for (const s of services()) {
    expect(operationCount(s), s.name).toBe(200);
  }
});

test("the four sizes are 200, 400, 600 and 1000", () => {
  expect([1, 2, 3, 5].map((n) => catalogOf(n).length)).toEqual([200, 400, 600, 1000]);
});

test("a shared noun appears in more than one service (axis B)", () => {
  // 注文 / 明細 / 承認 / 社員 / 取引先 each reach two services or more.
});

test("every service has three near-neighbour groups of four or more (axis C)", () => {
  // group keys, counted.
});
```

- [ ] **Step 3: run it and watch it fail** — `pnpm -C e2e vitest run narrowing`

- [ ] **Step 4: write the five service definitions**

Thirty resources, twenty aggregates, twenty settings and ten workflow actions
per service — 200 exactly. Authoring rules, which are the measurement:

- **Axis A comes free**: every `list` summarises as `<noun>の一覧`, so 一覧
  matches 180 of the thousand. Do not vary the verb wording.
- **Axis B**: five shared keys — `order` (受注 / 発注), `line` (受注明細 /
  発注明細 / 経費明細), `approval` (経費承認 / 発注承認 / 勤怠承認),
  `employee` (社員), `partner` (取引先). A shared resource's noun differs but
  **its `displayName` collapses to the shared word**: both 受注 and 発注 get
  `注文` in one of their operations, so 「注文を一覧」 has two defensible
  answers.
- **Axis C**: three groups per service, four or five resources each. Inventory:
  在庫品目 / 在庫ロット / 在庫引当 / 棚卸 / 在庫調整.
- **Axis E**: the twenty settings per service are written by hand so that each
  carries a transactional noun it does not answer for. At least ten across the
  whole fixture must be decoys the corpus will name, e.g.
  `getAttendanceOvertimeThreshold`, summary `残業時間の上限設定を取得`.
- **Axis D is a constraint, not content**: nothing to add here. It is satisfied
  by the corpus using words the fixture never wrote, and Task 3 asserts it.

- [ ] **Step 5: write the generator**

`toOpenAPI(fixture)` returns a document with `openapi: "3.0.3"`, an `info`
carrying `x-ui-hint.displayName`, one path item per resource path, and for
each operation: `operationId`, `summary`, `description`, `tags`,
`x-orchestra-expose: true`, `x-ui-hint.displayName`, and a response or request
body that refers to shared `components.schemas` (`FixtureRecord`,
`FixtureRecordList`, `NewFixtureRecord`). One `status` enum, reused, with
`x-enum-labels`.

Summaries are generated from the noun: `<noun>の一覧`, `<noun>を1件取得`,
`<noun>の作成`, `<noun>の更新`, `<noun>の削除`. A `Setting`'s summary is used
verbatim.

- [ ] **Step 6: write the failing contract test**

```ts
// e2e/narrowing/fixture/openapi.test.ts
test("operation ids are unique across all five services", () => {
  /* ... */
});
test("every exposed operation has an x-ui-hint.displayName", () => {
  /* ... */
});
test("every enum carries x-enum-labels of the same length", () => {
  /* ... */
});
test("every exposed operation has a response schema or a request body", () => {
  /* ... */
});
```

These are the rules `harness/guard/operation-ids.sh`,
`harness/guard/exposed-ops.sh` and the Redocly ruleset apply to
`services/*/api/openapi.yaml`. Redocly is not wired to the fixture (it is not a
service); the tests are how AC-T-103 is met.

- [ ] **Step 7: `make fmt && make check`, then commit**

```bash
git commit -- e2e/narrowing/fixture
```

---

### Task 2: the fixture served, and a catalogue the platform agrees with

**Files:**

- Create: `e2e/narrowing/serve.ts`
- Test: `e2e/narrowing/serve.test.ts`

**Consumes:** Task 1's `services()` and `toOpenAPI`.

**Produces:** one process that serves all five contracts, and a recorded
verification that the platform builds a 1000-operation catalogue from it.

- [ ] **Step 1: write the failing test** — a server started on port 0 answers
      `GET /<service>/openapi.yaml` with a YAML body that parses back to the same
      operation ids, for each of the five, and 404s for an unknown service.

- [ ] **Step 2: run it and watch it fail**

- [ ] **Step 3: write the server** — one `node:http` server, `NARROWING_PORT`
      (default 8090), no dependency beyond a YAML serialiser already in the
      workspace. It serves contracts only; **it does not implement the
      operations** (spec section 8).

- [ ] **Step 4: run the test until green**

- [ ] **Step 5: verify the platform agrees, by hand, once**

```bash
node e2e/narrowing/serve.ts &
ORCHESTRA_SERVICES="inventory=http://localhost:8090/inventory,sales=http://localhost:8090/sales,purchasing=http://localhost:8090/purchasing,attendance=http://localhost:8090/attendance,expense=http://localhost:8090/expense" \
ORCHESTRA_DB_PATH=/tmp/narrowing.db ORCHESTRA_ADMIN_PASSWORD=dev-only-admin-password \
  ./services/platform/bin/api
# then, signed in: GET /api/catalog | length == 1000
```

Record the number in the commit message. This is AC-T-101 and AC-T-102, and it
is the only step that runs the platform — **do not add it to any make target.**

- [ ] **Step 6: `make fmt && make check`, then commit**

---

### Task 3: the lexical baseline

**Files:**

- Create: `e2e/narrowing/lexical.ts`
- Test: `e2e/narrowing/lexical.test.ts`

**Consumes:** Task 1's fixture.

**Produces:** `index(catalog)` and `narrow(index, question, k)` returning the
top K operation ids with their scores. Task 4's corpus checks use it, which is
why it comes first.

- [ ] **Step 1: write the failing test**

```ts
test("an exact noun match out-ranks a shared verb", () => {
  // 在庫ロットの一覧 beats every other 〜の一覧 for the query 在庫ロット.
});

test("scoring 1000 operations takes under 5ms", () => {
  /* ... */
});

test("the index is built once and holds no state between queries", () => {
  /* ... */
});
```

- [ ] **Step 2: run it and watch it fail**

- [ ] **Step 3: write `lexical.ts`** — a character-bigram index over each
      operation's summary, description, display name and service display name,
      built when the catalogue is read. Score by bigram overlap normalised by query
      length. No tokeniser, no model, no network, nothing kept warm.

  It is the weakest thing that is not a strawman (spec section 7). Do not
  improve it with a stop-word list, a synonym table or a hand-tuned boost: its
  job is to be the floor, and a tuned floor is not one.

- [ ] **Step 4: run until green**

- [ ] **Step 5: `make fmt && make check`, then commit**

```bash
git commit -- e2e/narrowing/lexical.ts e2e/narrowing/lexical.test.ts
```

---

### Task 4: the corpus, the measurement, and the numbers

**Files:**

- Create: `e2e/narrowing/corpus/types.ts`
- Create: `e2e/narrowing/corpus/axis-a.ts` … `axis-e.ts`
- Create: `e2e/narrowing/corpus/index.ts`
- Create: `e2e/narrowing/measure.ts`, `e2e/narrowing/report.ts`
- Test: `e2e/narrowing/corpus/corpus.test.ts`,
  `e2e/narrowing/measure.test.ts`
- Modify: `Makefile` (one new target; protected path), `DECISIONS.md`

**Consumes:** Tasks 1 and 3.

**Produces:** `questions()` — 100 `Question` values, 25/25/25/15/10 — and a
report.

- [ ] **Step 1: write the type**

```ts
export type Axis = "A" | "B" | "C" | "D" | "E";

export interface Question {
  readonly id: string;
  readonly axis: Axis;
  /** The question, in Japanese, as a person would type it. */
  readonly text: string;
  /** Every operationId that would be a defensible answer. Axis B has several. */
  readonly answers: readonly string[];
  /**
   * Axis E only: the operation that is lexically closer than the answer and is
   * wrong. Asserted to actually out-score the answer, so the axis is measured
   * rather than claimed.
   */
  readonly decoy?: string;
}
```

- [ ] **Step 2: write the failing corpus checks** — these are what make the
      axes real, and they fail before the questions exist:

```ts
test("the split is 25/25/25/15/10", () => {
  /* ... */
});

test("every answer names an operation the fixture serves", () => {
  /* AC-T-105 */
});

test("axis B questions have two or more answers in different services", () => {
  /* ... */
});

test("axis D questions share no character bigram with their answer's text", () => {
  // The vocabulary gap, asserted. If a D question overlaps its answer's
  // summary, description or display name, it is not a D question.
});

test("axis E decoys out-score their answers", () => {
  // Task 3's scorer. If the decoy does not win, the question is not an E
  // question and the axis is not measuring what it says.
});
```

- [ ] **Step 3: write the questions**, per axis, following spec section 4.
      Axis D's fifteen use vocabulary the fixture never wrote — 品切れ, 休みたい,
      PO, 立て替え, 締め日. Axis E's ten each name a decoy. Iterate until the
      Step 2 checks pass: a D question that overlaps, or an E decoy that loses, is
      rewritten, not excused.

- [ ] **Step 4: write the failing measurement test** — `measure()` over a
      handful of questions returns recall per axis and overall for each K.

- [ ] **Step 5: write `measure.ts` and `report.ts`** — for sizes 1/2/3/5
      services × K of 10/20/50, report recall per axis, recall overall, and
      per-query wall-clock. One line per (size, K); axes as columns. **State the
      direction in the header** — the model comparison had to be rewritten once
      because a column did not say whether bigger was better (`DECISIONS.md`,
      2026-09-14).

- [ ] **Step 6: add the target** — `make narrowing`, depending on nothing,
      running `node e2e/narrowing/measure.ts`, not quiet (it prints its own
      report). **Not** added to `check`, `test`, `acceptance` or `lint`.
      Needs `ORCHESTRA_ALLOW_HARNESS_CHANGE=1` on this commit.

- [ ] **Step 7: run it, and record the numbers** — `DECISIONS.md`, dated, per
      axis, including the axes the baseline fails. AC-T-107. Do not editorialise
      the D column: the baseline is expected to score near zero there, and that is
      the measurement working, not a defect to fix.

- [ ] **Step 8: `make fmt && make check`, then commit**

---

## What comes after, and is not in this plan

Choosing the narrowing mechanism. Its input is Task 4's numbers: the K at which
the lexical baseline holds, and the per-axis shortfall. Until those exist, a
vector store is a guess.
