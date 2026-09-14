# What the model is offered

The thirteenth subproject. It changes one thing: a built-in tool can say
when it applies, instead of every request carrying every tool.

## 1. What it proves

That "always present" was a property of having only two of them.

`ask_user` and `list_capabilities` apply to any question anybody can ask, so
the tool list was assembled by concatenation and nobody had to decide
anything. `propose_panel` is the third, and it is the first that does not:
it puts a panel on a workspace, and a question asked from the chat screen
has no workspace to put one on. `docs/specs/proposing.md` N4 says so, and
enforces it in the browser - `ConversationPanel` passes no way to place a
proposal unless it knows a workspace id, so a proposal arriving at the chat
screen draws nothing.

The platform kept offering the tool anyway, and that is not free. Measured
(`DECISIONS.md`, 2026-09-14): `qwen38-27b-iq3s` answers plain questions like
出荷準備完了の在庫を見せて with a **panel proposal**, three times in ten -
a screen the person is not on, for a question about a table. The browser
drops it, so nobody sees a bug; what they see is a question that did not
get answered.

`docs/specs/proposing.md` section 9 predicted the shape of this and asked
for it to be measured. What it did not predict is that the answer would
depend on the model: the default answers this way rarely enough to sit
inside its own noise, and a different model does it three times in ten.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                       |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **O1** | A built-in tool declares when it applies. Not "is a workspace open" - a condition of its own, so the next tool with a different condition does not need a second mechanism.                    |
| **O2** | The condition is evaluated against what the request carries, once, where the tool list is built. A tool whose condition is not met is not in the list the model sees.                          |
| **O3** | `propose_panel` applies when the question was asked from a workspace. That is the only condition anything needs today; O1 exists so the second one is an entry in a list rather than a design. |
| **O4** | The request says where it was asked from. `POST /api/plan` gains an optional workspace id - what the browser already knows and already uses to decide whether a proposal can be drawn at all.  |
| **O5** | A tool the model calls anyway is refused, as an unknown tool would be. The list is what the model is offered, not what the platform trusts.                                                    |

## 3. What a built-in tool becomes

```
BuiltinTool
  tool       the definition, as today
  applies    func(PlanContext) bool
```

`PlanContext` is what the request carries that a condition can read - today,
whether a workspace id came with it. It grows when a condition needs
something new, and a tool that applies to everything says so by having no
condition at all, which is what `ask_user` and `list_capabilities` do.

The list is built where it always was (`usecase.ToolsFor`), and the only
change at the call site is that it now takes the context.

## 4. Why not "is a workspace open"

That is the condition `propose_panel` happens to have, and writing it into
the mechanism would mean the fourth tool - one that applies only to an admin,
or only when a conversation has history, or only when some service is
reachable - arrives as a second flag beside the first.

Three tools is exactly the point where this is cheap to get right and still
obvious what it is for. Naming the general thing now costs one function
signature; naming it later costs every call site.

## 5. Deliberately excluded

- **Per-operation conditions.** An endpoint reaches the catalogue through
  `x-orchestra-expose` and is narrowed by permission (`docs/specs/auth.md`
  A4); those are two mechanisms that already exist and neither needs this.
- **Conditions the model can influence.** A condition reads the request, not
  the conversation's content. A tool that appears because the model said
  something is a tool the model can talk its way into.
- **Telling the model why a tool is absent.** An absent tool is absent. A
  sentence explaining that it would have been available elsewhere is prompt
  the model did not need and will sometimes act on.

## 6. Acceptance criteria

- **AC-O-101** A question asked with no workspace is offered no
  `propose_panel`, and the tools it is offered are otherwise identical.
- **AC-O-102** A question asked from a workspace is offered it.
- **AC-O-103** `ask_user` and `list_capabilities` are offered in both cases -
  a tool with no condition is unaffected.
- **AC-O-104** A `propose_panel` call arriving when the tool was not offered
  is refused the way an unknown tool is, and nothing is written.
- **AC-O-105** The catalogue's own operation tools are unchanged by any of
  this - the same tools, in the same order, byte for byte.

## 7. What this does not fix

The measurement that prompted it was `qwen38-27b-iq3s` proposing a panel for
a question about a table. This stops that happening from the chat screen. It
does **not** stop it happening on a workspace screen, where the tool does
apply and the model may still reach for it when a person asked to see a
table.

Whether that is a problem is a question for the corpus, which today asks
every question without a workspace and so will stop seeing this entirely.
A case that asks from a workspace is what would keep watching it, and there
is not one yet.
