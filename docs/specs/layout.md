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

|        | Decision                                                                                                                                                                                                                                                                  |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **L1** | A panel's size is a span in grid columns and rows, never pixels. A dashboard is a grid; a panel sized in pixels is a panel that is wrong on the next screen.                                                                                                              |
| **L2** | Size and order are changed by controls, not by dragging. See section 4 - this is the decision this subproject exists to argue, and the one a reader will want the reasoning for.                                                                                          |
| **L3** | `position` is the order. `docs/specs/workspaces.md` W5 added the column and excluded the editing; this is where the editing lands, and no new column is needed for it.                                                                                                    |
| **L4** | The grid is CSS grid, from MUI's `Box` - twelve columns where there is room for them, one where there is not. A panel's span is clamped to what fits, so a wide panel on a phone is a full-width panel rather than a sideways scroll `make guard-layout` would fail over. |
| **L5** | Size and order are saved through the `PATCH` that already exists (`docs/specs/dashboard.md` P11). A panel's geometry is a field of the panel, not a separate document about where panels go.                                                                              |

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

## 4. Why controls and not dragging

This is L2, and it is the whole argument of this subproject.

Dragging is what Grafana does and what a person pictures when they hear
"arrange". It also needs a grid engine - collision, reflow, a drag preview,
pointer capture, touch, autoscroll at the edges - which MUI does not have.
`react-grid-layout` is the library that does, and adopting it is a decision
with three costs worth naming rather than a line in `package.json`:

- **A dependency this repository's own precedent argues against.**
  `web/src/app/model/useHashRoute.ts` rejected `react-router` in these
  terms: a dependency, a new tree, and a new surface for
  `make guard-fsd`/`guard-ui` to reason about, to solve a problem thirty
  lines solved. A grid engine is bigger than a router.
- **A11y.** `make guard-a11y` runs axe against every screen
  (`harness/quality/browser/screens.ts`). A drag handle is not operable from
  a keyboard unless somebody makes it so, and a grid engine's own handles
  are mouse-first. Controls are buttons, and a button is operable by
  construction.
- **It is not what was asked for.** What a person wanted was a wide panel
  and a different order. Dragging is one way to say that; two buttons and a
  size control are another, and they are the ones that work on a phone.

So: a panel's card carries a control for its width, one for its height, and
a way to move it earlier or later. Every one of them is a button or a
select, measured by both browser gates like everything else.

**Dragging is not refused, it is deferred**, and the argument above is what
it has to answer. Once a workspace is worth arranging - which is what this
subproject makes true - somebody can weigh a grid engine against a screen
that already works without one, with a11y as the thing to beat rather than
the thing to discover.

## 5. The grid

```
wide      12 columns, a panel spans width (clamped 1-12)
narrow     1 column,  every panel spans it
```

One breakpoint, MUI's own `sm`. Not a per-breakpoint layout per panel: that
is three numbers per panel for a product nobody has asked to lay out
differently on a tablet, and each one is a thing to keep in sync.

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

- **Dragging and free resizing.** Section 4 argues it, and says what it
  would have to answer.
- **A layout per screen size.** Section 5 argues it.
- **Overlapping panels.** A grid that allows two panels in one cell needs a
  collision rule, and a collision rule is the grid engine section 4 defers.
- **A panel taller than the rows it is given.** A panel whose content
  overflows scrolls inside its own card, as it does today; growing to fit
  would move everything below it and make one refresh relayout a screen.

## 8. Acceptance criteria

- **AC-L-101** A panel's width and height can be changed, and it draws at
  that size after a reload.
- **AC-L-102** A panel can be moved earlier and later, and the order after a
  reload is the order on screen.
- **AC-L-103** Moving the first panel earlier, or the last later, does
  nothing and reports nothing - there is no error to show for a thing that
  cannot happen.
- **AC-L-104** A panel saved before this slice - no width or height at all -
  draws full width and one row tall.
- **AC-L-105** At 375px every panel spans the single column, and the page
  does not scroll sideways.
- **AC-L-106** A wide panel draws a wider chart, not the same chart in a
  wider box.

## 9. Harness work this implies

None expected. `harness/quality/browser/screens.ts` already measures a
workspace, and a workspace with panels of several sizes is the same screen
with more on it - though it is worth checking whether that gate should seed
a panel, since today it measures an empty workspace
(`DECISIONS.md`, 2026-09-13).
