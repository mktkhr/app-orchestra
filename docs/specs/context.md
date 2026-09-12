# Multi-turn context

The fourth subproject. It changes one sentence of
`docs/specs/orchestration.md` and adds no state to the platform.

## 1. What it proves

That a conversation is a conversation. "検品保留の在庫を見せて" works today;
"勤怠でも同じことして" does not, because the second question arrives with no
memory of the first and the model has nothing to resolve 同じこと against.

It also proves D8 was two sentences pretending to be one. "One request is one
LLM call" is about cost and stays. "The API result never goes back to the LLM"
is about trust and also stays. What was never argued for is the model not
knowing what it was asked a minute ago.

## 2. Decisions taken here

|        | Decision                                                                                                                                                |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **M1** | The model sees earlier questions and what the platform decided for each - service, operation, arguments. It never sees a row of the answer.             |
| **M2** | The browser keeps the conversation and sends it. The platform stays stateless; nothing is stored and nothing is scoped to an owner.                     |
| **M6** | A conversation has a name and a lifetime of its own, held above the screen that shows it. It ends when somebody ends it, not when a component unmounts. |
| **M3** | The turns go after the catalogue in the prompt, never before it. The catalogue is what the cache is warm for.                                           |
| **M4** | A window, not a transcript. The most recent turns, bounded by a configured count, because a conversation has no end and a prompt does.                  |
| **M5** | Both planners take the same turns. The catalogue is rendered two ways already; this is rendered two ways too.                                           |

## 3. What a turn is

```
Turn
  question     "検品保留の在庫を見せて"
  kind         result | form | ask | none
  service      "inventory"      (absent for none)
  operationId  "ListInventoryItems"
  args         {"status": "quarantined"}
```

Every field is something the platform already told the browser: `source` on a
result, `target` on a form, and the question the person typed. Nothing is
derived and nothing is remembered that was not already on screen.

The answer's data is not there. That is M1, and it is the half of D8 that was
always about trust rather than cost: a model that has seen the rows will talk
about the rows, and some of what it says will be wrong in a way nobody can see.
A model that has seen only the call it made can only tell you what it did.

## 3a. What a conversation is

Today a conversation is `useState` inside the component that draws it, which
means its lifetime is an accident of rendering: opening a workspace unmounts
the chat and the conversation is gone, and the workspace screen has a second
conversation of its own that nobody named. Nothing decided either of those; the
router did.

That is survivable while the turns are only drawn. It stops being survivable
when they are sent: a conversation that ends without being ended is a screen
going blank, which a person notices, and one that continues when it should not
is a model answering with context nobody meant to give it, which nobody sees.

So a conversation is keyed and lives above the screen - `"chat"`, and one per
workspace id. Switching screens and coming back finds the same conversation.
Ending one is a thing a person does, through a control that says so, and it is
the only thing that empties it.

Not stored, still (M2): a reload starts fresh, which is the one boundary a
person can always see.

## 4. Why it goes after the catalogue

`PRODUCT.md` D2 sends the whole catalogue on every request and keeps it warm in
the prompt cache. A cache is warm for a prefix: anything that changes has to sit
behind everything that does not. The catalogue does not change between two
questions in a conversation; the turns do. So the turns go last, and the
catalogue is still the same bytes it was.

Putting them first would invalidate the cache on every question, which is the
cost D2 spent a decision avoiding.

## 5. Contract

`POST /api/plan` gains one optional field:

```
{ query, answers?, turns? }
```

`turns` is what section 3 describes, oldest first. Absent or empty means what it
means today: a question with no history.

Nothing else changes. `/api/invoke` has no use for it - a person pressing a
button is not asking a question.

## 6. The window

`ORCHESTRA_CONTEXT_TURNS`, defaulting to a small number. The browser sends what
it has; the platform keeps the most recent that many and drops the rest, so a
long afternoon does not become a long prompt.

The platform truncates rather than the browser, because the limit is about what
the model can be told and the platform is what knows the model.

## 7. Deliberately excluded

- **Storing the conversation.** `docs/requirements.md` names
  `Conversation / Message`, and the browser already holds both. Adding tables
  would buy surviving a reload, and cost a conversation list, a way to switch
  between them, and an owner on every row. That is its own subproject. What is
  not deferred is a conversation knowing what it is (M6) - that had to be
  settled here, because sending the turns is what makes the difference between
  two conversations matter.
- **Summarising the answer in prose** (FR-C-6). It needs the rows, which is the
  thing M1 refuses, and `docs/specs/orchestration.md` section 13 already
  excluded it for doubling the calls.
- **Referring to a row.** "その2つ目を登録して" cannot work without the rows.
  What the catalogue can express, the model can ask for again.

## 8. Acceptance criteria

- **AC-M-101** A question following one about inventory, phrased with no service
  name, is answered from the service the previous question used.
- **AC-M-102** The tools the planner is offered are unchanged by the turns; only
  the conversation before the question differs.
- **AC-M-103** No row of any previous answer appears in what is sent to the
  model.
- **AC-M-104** A conversation longer than the window sends only the most recent
  turns, oldest dropped first.
- **AC-M-105** A question with no turns is answered exactly as it is today.
- **AC-M-107** Opening a workspace and returning to the chat finds the same
  conversation, and the workspace's own conversation is a different one.
- **AC-M-108** Ending a conversation empties it, and the next question is sent
  with no turns at all.
- **AC-M-106** The JSON planner takes the same turns and reaches the same
  decision shape as the tool-calling one.

## 9. Harness work this implies

None expected. Nothing is stored, no dependency is added, and `make check` still
needs nothing running.
