# Picking something to put on a dashboard

The ninth subproject. It adds no capability: everything it touches can
already be done. What it changes is how long it takes.

## 1. What it proves

That the right fix comes from asking what is slow, not from the roadmap.

`docs/requirements.md` FR-B-2 and FR-B-6 describe a genre layer - services
grouped by what they are for, sourced from OpenAPI `tags` - and it was next
on the list when somebody said the builder was already hard to use with six
operations in it.

It would not have helped. Every exposed operation in `services/inventory`
carries the single tag `items`, and every one in `services/attendance`
carries `records`, so grouping by tag produces exactly one group per
service - which is what the picker already groups by. The feature would have
worked, changed the screen not at all, and the only way to make it look
useful would have been to invent more tags for the dummy services, which is
shaping the evidence to fit the conclusion.

Asked what was actually slow, the answer was two things: finding the
operation, and the length of the form after it.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                          |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **K1** | The picker searches everything a person might type, not only what it displays: the operation's own name, its service's name, its summary, and the names of the fields it returns.                                                                 |
| **K2** | An option says what it is. The name, and under it the summary - the sentence the contract already writes and the model already reads. A list of names alone asks a person to know the names.                                                      |
| **K3** | The form starts at its shortest and grows as choices are made. Not a wizard (section 6 of `docs/specs/dashboard.md` still holds - every control that applies is on screen at once) - but a control that cannot apply yet is not on screen either. |
| **K4** | The genre layer is deferred, not dropped, and the reason is written down: with these contracts it groups nothing. It becomes worth building when a service carries more than one tag over its exposed operations.                                 |

## 3. What a person types

Today the picker filters on its own label, which since `x-ui-hint.displayName`
is a short Japanese name. So 在庫 finds things and 数量 finds nothing, though
the field is right there in the result.

What it matches (K1):

```
在庫一覧                    the display name
在庫管理                    the service's display name
ListInventoryItems          the operation id
List stock items...         the summary
数量  ステータス  品名       the titles of the fields it returns
```

The last line is the one that matters. A person building a dashboard is
usually thinking about the number they want to see, not the name somebody
gave the endpoint that returns it.

Matching is plain substring, case-insensitive, over those strings joined.
Not fuzzy: a fuzzy match on six items is indistinguishable from a substring
match, and on six hundred it is a ranking problem this does not have yet.

## 4. What an option shows

```
在庫一覧
List stock items, optionally filtered by status.
```

The summary is English in these contracts, because it is the tool
description the model reads (`docs/specs/orchestration.md` D15) and
translating it would move `make eval`'s numbers. That is a real wart and it
is the contract's to fix, not this screen's: a service that wants a Japanese
sentence under its name can write `x-ui-hint.displayName` today and a
Japanese `summary` the day somebody measures what that does to the planner.

## 5. What the form shows, when

`docs/specs/dashboard.md` section 6 lists five steps and insists they are
one form rather than a wizard. That stands. What changes is that a control
with nothing to decide yet is absent rather than empty:

```
before an operation is picked     the picker, and nothing else
after                             its arguments, how to draw it, its name
when "chart" is chosen            the axes
when the transform is switched on  its own three fields
```

Three of those four already behave this way. The one that does not is the
middle: picking an operation reveals every remaining control at once,
including a transform section for an operation whose rows nobody has asked
to group.

So the transform starts collapsed behind its own switch, which it already
has, and the switch is what is on screen - one line instead of four.

## 6. Deliberately excluded

- **A genre layer** (K4).
- **Fuzzy or ranked search.** Section 3 argues it.
- **Searching the rows.** The catalogue knows what fields an operation
  returns; it does not know what is in them, and asking every service on
  every keystroke is a different product.
- **Remembering what somebody picked last time.** A reasonable thing to want
  and a different decision, about storing a person's habits, which this
  product has nowhere to put yet.

## 7. Acceptance criteria

- **AC-K-101** Typing the title of a field an operation returns finds that
  operation, and typing a word in no operation finds none.
- **AC-K-102** Each option shows its name and its summary.
- **AC-K-103** Before an operation is picked, the form shows only the
  picker. After, it shows the arguments, the component and the name - and
  the transform as a single switch, not its fields.
- **AC-K-104** Everything `docs/specs/dashboard.md` section 10's criteria
  say about building a panel still holds: this changes what is on screen,
  not what a panel is.

## 8. Harness work this implies

None expected. `make guard-a11y` and `make guard-layout` already measure the
panel builder (`harness/quality/browser/screens.ts`), and a shorter form is
the same screen with fewer controls on it.
