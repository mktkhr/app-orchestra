# Asking for a panel

The tenth subproject, and `docs/requirements.md` FR-F-5: the workspace
screen's own chat can be told, in Japanese, to put something on the
workspace.

## 1. What it proves

That the model can compose a panel without being allowed to place one.

Asking for a panel is already possible in two steps: ask a question, like
the answer, press save. What that cannot do is the half a person actually
says out loud - 「在庫をステータス別に棒グラフで置いて」 - because the
answer comes back as a table and the axes, the grouping and the chart are
things the person then sets by hand in a form.

So the model fills the panel in, and a person presses once.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                  |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **N1** | The model proposes a panel; it never places one. There is no tool that writes to a workspace. `docs/specs/orchestration.md` D8 says a person presses the button, and this is the same button.             |
| **N2** | A proposal is a plan result with a panel attached - the operation, its arguments, the component, and the view. Everything a panel already is (`docs/specs/dashboard.md` section 3), and nothing new.      |
| **N3** | The person sees what will be placed before placing it, and can change it. The proposal opens the builder's own form, filled in. Accepting is one press; the form is there because the model can be wrong. |
| **N4** | Only the chat on a workspace screen can produce one. The chat screen has no workspace to put anything on, and inventing one silently is worse than not offering.                                          |
| **N5** | Changing an existing panel by asking is not in this slice. The model would have to be told what is already there, which is a second thing to put in the prompt and a second thing for it to get wrong.    |

## 3. What the model is offered

One further built-in tool, alongside `ask_user` and `list_capabilities`
(`docs/specs/orchestration.md` D11, D14):

```
propose_panel(service, operationId, args, component?, chart?, transform?, title?)
```

It is a tool the model **answers with**, not one that does anything. The
platform turns the call into a result the browser draws, exactly as it turns
`ask_user` into a question. Nothing is written, nothing is called, and the
workspace is unchanged until a person says so.

`component`, `chart` and `transform` are the panel's view
(`docs/specs/dashboard.md` P1). The model may leave all of them out, in
which case the proposal is whatever the rendering rule would have drawn -
which is the two-step flow it replaces, with the form already open.

## 4. What comes back

```
{ kind: "proposal", panel: { service, operationId, args, component, view?, title } }
```

A fifth `kind` beside `result`, `form`, `ask` and `none`. Not a `result`
with an extra field: a result is an answer to a question and this is an
offer, and a browser that told them apart by looking for a field would be
one refactor away from drawing the wrong thing.

The platform fills in what the model left out, from the catalogue, so the
browser never has to: the component from `domain.Render`, the axes from
`x-ui-hint.chart` when the contract declares them (`docs/specs/orchestration.md`
D15), the title from the operation's display name. The model's own values
win where it gave them.

Refused the same way everything else is: a proposal naming an operation the
person may not call is not produced at all, because the catalogue the
planner was offered never held it (`docs/specs/auth.md` A4).

## 5. What the person sees

The proposal draws in the conversation, where the answer would have been:
the panel's title, what it will call, and the form that would have built
it - already filled in - with one control that places it.

The form is the builder's own (`docs/specs/dashboard.md` P12, P7). A
proposal is a panel somebody has not agreed to yet, and the place to
disagree with one is the same screen that builds them by hand.

Placing it is `POST /api/workspaces/{id}/panels`, the endpoint that already
exists. The model is not in that request.

## 6. Deliberately excluded

- **A tool that writes.** N1.
- **Changing or removing an existing panel.** N5.
- **Proposing from the chat screen.** N4.
- **Proposing several panels at once.** "ダッシュボードを作って" is a
  reasonable thing to say and a different feature: it needs the model to
  decide what a whole screen should hold, and this one is about one panel a
  person described.

## 7. Acceptance criteria

- **AC-N-101** Asking the workspace's chat for a panel produces a proposal
  naming the operation, the arguments and the view the question described.
- **AC-N-102** Nothing is written and no service is called until the person
  places it; refusing the proposal leaves the workspace exactly as it was.
- **AC-N-103** A proposal the person edits before placing is placed as
  edited.
- **AC-N-104** The chat screen's own conversation offers no proposal.
- **AC-N-105** A proposal never names an operation the person may not call.
- **AC-N-106** `make check` still calls no model: the proposal path is
  driven by the stub planner like every other decision.

## 8. Harness work this implies

None expected. The tool is one more in the list `usecase.ToolsFor` builds,
the result is one more `kind` on a contract that already has four, and the
stub planner already answers with whichever the fixture names.

## 9. What this costs the planner

One more tool in every request's tool list, on every question, whether or
not a workspace is open. `PRODUCT.md` D2 sends the whole catalogue every
time and keeps it warm; one tool definition is small beside it, and
`make eval`'s corpus is what says whether offering it changes how the model
answers questions that have nothing to do with panels. Measure before and
after: a tool that quietly makes `no-enum-value` worse is a tool that cost
more than it looks.
