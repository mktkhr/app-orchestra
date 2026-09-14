# A conversation that looks like one

The twelfth subproject. It changes no behaviour: the same questions reach
the same planner and the same answers come back. What changes is that the
screen looks like the thing it is.

## 1. What it proves

That "chat-like" is a shape, and this had the parts without the shape.

A question draws as a full-width panel with a tinted background; an answer
draws as whatever it is, with nothing around it. Side by side they read as
two paragraphs of a document, not as two people taking turns. And while the
model is thinking - which against a local model is seconds, not
milliseconds - the screen shows nothing at all in the place the answer will
appear. `pending` exists and is spent entirely on disabling the form.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                           |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **C1** | A question is a bubble on the right. That is what every person who has used a chat expects of their own words, and expectation is the whole of what this subproject is buying.                                                                     |
| **C2** | **A rendered result is not a bubble.** A table, a form, a chart or a proposal takes the width it needs, on the left, with the turn's own spacing around it. A bubble is for a sentence.                                                            |
| **C3** | An answer that _is_ a sentence - `kind: "none"`'s message, an error - is a bubble on the left. The rule is about what the thing is, not about who said it.                                                                                         |
| **C4** | While a question is in flight, the answer's place holds a spinner. In the answer's place, not beside the form: the eye is already there, waiting for the reply.                                                                                    |
| **C5** | The spinner claims nothing about progress. `docs/requirements.md` FR-E-5 wants to show which service is being selected; one request is one LLM call (D8) and the platform knows no intermediate state to report. A bar that filled would be lying. |

## 3. What a turn looks like

```
                                   ┌─────────────────────┐
                                   │ 検品保留の在庫を見せて │   question, right
                                   └─────────────────────┘

┌───────────────────────────────────────────────────────────┐
│ a table, full width, as it draws today                    │   result, left
└───────────────────────────────────────────────────────────┘

┌──────────────────────────┐
│ どのサービスも答えられません │                                 sentence, left
└──────────────────────────┘
```

A question's bubble is bounded - it stops well short of the full width, so
the alternation is visible even when every message is short. A long question
wraps rather than stretching.

## 4. Why a result is not a bubble

C2 is the decision somebody will want to undo, so the reason is here.

This product's answers are not text. They are tables with pagination, forms
with a dozen controls, charts sized from the box they are in
(`docs/plans/dashboard.md` Task 5), and panel proposals carrying the whole
builder. Putting those inside a rounded, inset, max-width container would
take width away from the one thing on screen that needs it, and would put a
speech bubble around a form nobody spoke.

So the bubble is a treatment for a sentence, and a result keeps the space it
has today. What makes the conversation legible is the **alternation** - the
person's words on the right, everything else starting from the left - not a
uniform frame around each turn.

## 5. The spinner

In the answer's position, left, from the moment a question is sent until its
answer replaces it. It is a spinner and a short line of text, and the text
does not pretend to know more than the platform does (C5).

It is removed when the answer arrives or the request fails - a failure
already draws its own message, and a spinner that outlives its request is
worse than none.

## 6. Deliberately excluded

- **Showing which service is being chosen** (FR-E-5). C5 argues it. It
  becomes possible if the platform ever streams, which is its own decision
  about D8.
- **Avatars, names, timestamps.** One person and one assistant; a label
  saying which is which is a label nobody reads twice.
- **Changing what any answer renders.** This is a container and a placeholder.

## 7. Acceptance criteria

- **AC-C-101** A question draws on the right, bounded, and wraps rather than
  stretching when it is long.
- **AC-C-102** A table, form, chart or proposal answer takes the same width
  it takes today.
- **AC-C-103** A sentence answer - `kind: "none"` - draws as a bubble on the
  left.
- **AC-C-104** From sending a question until its answer arrives, a spinner
  occupies the answer's position; it is gone once the answer is drawn.
- **AC-C-105** A failed request leaves no spinner behind.
- **AC-C-106** At 375px the alternation is still visible and nothing scrolls
  sideways.

## 8. Harness work this implies

None expected. `make guard-a11y` and `make guard-layout` already measure the
chat screen. A spinner needs a name a screen reader can read, and MUI's own
`CircularProgress` takes one - `web/src/features/permissions/ui/AccountList.tsx`
already does this.
