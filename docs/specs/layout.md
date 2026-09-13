# Arranging a workspace

The seventh subproject, and the second half of what `docs/requirements.md`
FR-F calls a dashboard. `docs/specs/dashboard.md` built panels a person
makes and changes; this one lets them decide how big each is and what order
they come in (FR-F-4).

## 1. What it proves

That a workspace is a thing a person composes, not a column of cards in the
order they happened to be made.

Six panels of equal width, stacked in creation order, is not a dashboard.
The one number somebody watches all day should be able to be wide; three
small tables should be able to sit side by side; the panel that matters most
should be able to be first. None of that needs a drag engine, and all of it
needs somewhere to record the answer.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                                                                                                                  |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **L1** | A panel's size is a span in grid columns and rows, never pixels. A dashboard is a grid; a panel sized in pixels is a panel that is wrong on the next screen.                                                                                                                                              |
| **L2** | Panels are dragged and resized, through `react-grid-layout`. Section 4 records that this decision was first taken the other way and reversed, and what the reversal costs - a keyboard has to be able to do everything a drag can, which is now a condition of the work rather than a reason to avoid it. |
| **L3** | `position` is the order. `docs/specs/workspaces.md` W5 added the column and excluded the editing; this is where the editing lands, and no new column is needed for it.                                                                                                                                    |
| **L4** | Twelve columns where there is room for them, one where there is not. A panel's span is clamped to what fits, so a wide panel on a phone is a full-width panel rather than a sideways scroll `make guard-layout` would fail over.                                                                          |
| **L5** | Size and order are saved through the `PATCH` that already exists (`docs/specs/dashboard.md` P11). A panel's geometry is a field of the panel, not a separate document about where panels go.                                                                                                              |

## 3. What a panel carries now

```
Panel
  ...                     as docs/specs/dashboard.md section 3 has it
  position   0            the order, ascending (W5's own column)
  width      6            columns, 1-12
  height     1            rows
```

`width` and `height` are absent on every panel saved before this slice, and
default to what those panels draw as today: full width of the single column
they are stacked in, and one row tall.

## 4. Dragging, and the argument this section used to make

This section first argued the opposite, and the record of that is more
useful than a clean page.

It said: dragging needs a grid engine MUI does not have, and
`react-grid-layout` costs a dependency, costs accessibility, and is not what
was asked for - so a width control and two move buttons instead. Two of
those three did not survive contact with the person using the product.

**"Not what was asked for" was wrong.** What was asked for was
「Grafanaのようにパネルのサイズ変更とか出来ないし」. Dragging is what that
sentence means. A size select delivers the capability and not the thing.

**The precedent was wrong, and worse, it was the wrong precedent.**
`web/src/app/model/useHashRoute.ts` was cited: it rejected `react-router` on
the grounds that thirty lines solved the problem. Reading it again for this
argument showed that its own reasoning is about whether to take a router
library, while the decision it actually made - hash routing rather than real
paths - is a different question it never argues. The real reason for the hash
is that `internal/infra/httpserver/router.go` puts `http.FileServer` at "/"
with no fallback, so a real path 404s on reload. That is a server the
repository could change; nobody argued it should not. A precedent that turns
out not to have argued its own decision cannot carry another one.

**The accessibility cost was right, and it stays.** `make guard-a11y` runs
axe against every screen (`harness/quality/browser/screens.ts`), and a drag
handle is not operable from a keyboard unless somebody makes it so. A grid
engine's handles are mouse-first. This was the reason to avoid the library;
it is now a condition on using it: **every arrangement a drag can express, a
keyboard can express too**, and the gate is what says so. Section 8's
criteria name it.

The honest shape of the reversal: the first version optimised for not adding
a dependency, which is a cost the repository pays once, over doing the thing
somebody asked for, which they pay every day. The second version takes the
dependency and owes the keyboard.

## 5. The grid

```
wide      12 columns, a panel spans width (clamped 1-12)
narrow     1 column,  every panel spans it, dragging off
```

One breakpoint, MUI's own `sm`. Not a per-breakpoint layout per panel: that
is three numbers per panel for a product nobody has asked to lay out
differently on a tablet, and each one is a thing to keep in sync. On the
narrow one there is nothing to arrange - one column, one order - so the grid
is static there and the page cannot be dragged sideways by accident.

`height` multiplies a row's own height rather than naming a pixel count, so
a tall panel is tall in the same units as everything beside it. A chart
reads its own size from the card it is in (`docs/plans/dashboard.md` Task 5
made `ResultChart` take one), so a panel made wider draws a wider chart
rather than the same chart in a wider box.

## 6. Contract

```
PATCH /api/workspaces/{id}/panels/{panelId}   + position, width, height
GET   /api/workspaces/{id}                    panels carry width, height
POST  /api/workspaces/{id}/panels             + width, height (optional)
```

Nothing else changes. Reordering is `position` on the panels that moved -
the platform does not renumber a whole workspace on one move, because a
request that rewrites six rows to change one is a request that can lose the
other five.

## 7. Deliberately excluded

- **Free-form pixel sizes.** A panel is dragged and resized, but it lands on
  the grid (L1): the engine snaps, and a panel is always a whole number of
  columns and rows. A layout nobody can describe in the contract is a layout
  the platform cannot store.
- **A layout per screen size.** Section 5 argues it.
- **Overlapping panels.** A grid that allows two panels in one cell needs a
  collision rule, and a collision rule is the grid engine section 4 defers.
- **A panel taller than the rows it is given.** A panel whose content
  overflows scrolls inside its own card, as it does today; growing to fit
  would move everything below it and make one refresh relayout a screen.

## 8. Acceptance criteria

- **AC-L-101** A panel's width and height can be changed, and it draws at
  that size after a reload.
- **AC-L-102** A panel can be dragged to a new place and resized by its
  handle, and the arrangement after a reload is the arrangement on screen.
- **AC-L-103** Every arrangement a drag can reach, a keyboard can reach -
  moving a panel and resizing it are both operable without a pointer, and
  `make guard-a11y` passes on a workspace that has panels in it.
- **AC-L-104** A panel saved before this slice - no width or height at all -
  draws full width and one row tall.
- **AC-L-105** At 375px every panel spans the single column, and the page
  does not scroll sideways.
- **AC-L-106** A wide panel draws a wider chart, not the same chart in a
  wider box.

## 9. Harness work this implies

`harness/quality/browser/screens.ts` measures an empty workspace today
(`DECISIONS.md`, 2026-09-13). AC-L-103 is a claim about a workspace with
panels in it and a keyboard, so that gate has to seed a panel to be
evidence of anything here - a harness change, argued in `DECISIONS.md` and
committed with `ORCHESTRA_ALLOW_HARNESS_CHANGE=1`.

`react-grid-layout` ships its own CSS. This application is bundled, so
importing it is a bundler import and not a CDN request, but it is still
stylesheet the design system did not write: check what it paints and
whether `make guard-layout`'s contrast rules still hold under both colour
schemes.
