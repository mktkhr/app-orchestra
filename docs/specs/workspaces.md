# Workspaces

The second subproject. `docs/specs/orchestration.md` is the first; this one
assumes it and reuses almost all of it.

## 1. What it proves

That an answer worth keeping can be kept. The chat answers a question once and
the answer scrolls away; a workspace is where a person puts the answers they
want to look at again, and opening it asks the same questions over.

It also proves the catalogue was the right shape. A panel is a saved call -
service, operation, arguments - which is exactly what a result already tells
the browser it came from. Nothing new has to be remembered about an answer to
be able to repeat it.

## 2. Decisions taken here

|        | Decision                                                                                                                                                  |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **W1** | A panel is a saved call: service, operation id, arguments, component, title. Opening a workspace re-runs each one through `POST /api/invoke`.             |
| **W2** | Refreshing a panel is running it again. There is no endpoint for it and no cached result; the panel holds the question, not the answer.                   |
| **W3** | Workspaces live in a SQLite file the platform owns. Not in memory, because a person assembled them; not in a server, because nothing else here needs one. |
| **W4** | A panel is added by a person pressing a button. The planner is not changed and learns nothing about workspaces.                                           |
| **W5** | Panels are ordered by a stored position and drawn in that order. Dragging them about is not in this slice, but the column that would record it is.        |
| **W6** | A workspace has an owner column filled with the stub user. Authentication fills it in later; nothing else changes.                                        |

## 3. What a panel is

```
Panel
  id            assigned by the platform
  workspace_id
  service       "inventory"
  operationId   "ListInventoryItems"
  args          {"status": "quarantined"}
  component     "table"
  title         "検品保留の在庫"
  position      0
```

Every field but `title` and `position` is already in a plan result's `source`
and `component`. Saving an answer copies them; nothing is derived, and nothing
about the answer itself is stored.

That is W2's whole argument. A workspace that stored rows would be showing
yesterday's stock this morning, and a panel that stores the call cannot. The
cost is that opening a workspace with six panels makes six calls, which is
what a dashboard does.

## 4. Where it sits

The platform gains one repository and one usecase; the catalogue, the planner
and the rendering rule are untouched.

```
internal/domain        Workspace, Panel. Pure, like Catalog.
internal/usecase       WorkspaceStore port; the usecase that reads and writes it.
internal/adapter       repository/sqlite implements the port.
internal/infra/config  ORCHESTRA_DB_PATH.
```

`POST /api/invoke` already executes a named call with named arguments and is
already the one mouth an unsafe method goes through. A panel refresh is that
request, sent by the workspace screen instead of by a form. When the permission
check arrives it covers panels for free, because there is nowhere else to put
it.

## 5. Contract

```
GET    /api/workspaces                      -> [{ id, name, panelCount }]
POST   /api/workspaces                      { name } -> { id, name }
GET    /api/workspaces/{id}                 -> { id, name, panels: [Panel] }
DELETE /api/workspaces/{id}
POST   /api/workspaces/{id}/panels          { service, operationId, args, component, title } -> Panel
DELETE /api/workspaces/{id}/panels/{panelId}
```

No endpoint returns a panel's data. The browser opens a workspace, reads its
panels, and posts each to `/api/invoke` itself - the same call the chat makes
when a person submits a form.

## 6. Storage

SQLite, one file, named by `ORCHESTRA_DB_PATH`. `modernc.org/sqlite`, which is
pure Go: cgo would put a C toolchain between the repository and `make check`.

The schema is applied at startup from an embedded statement. At this size a
migration tool is a dependency to explain rather than a problem to solve; when
the schema changes twice in one week, that is the signal to add one.

Tests get a file in `t.TempDir()`. The e2e suite gets one in a temporary
directory too, so the suites cannot see each other's workspaces and `make check`
still needs nothing running.

## 7. Adding a panel

Two ways in, one of them the other's front door.

**From the chat.** A result carries its provenance already. The turn gains a
control that posts that provenance as a panel. The person says which workspace.

**From the workspace.** The workspace screen has the same chat. A question there
is answered the same way - `/api/plan`, unchanged - and the answer is drawn
where it was asked, with the same control under it.

So "在庫の表を追加して" works, and the planner never hears the word workspace.
It chooses an operation, which is the only thing it is good at; a person decides
whether the answer is worth keeping, which is the only part a model would be
guessing at.

The alternative was an `add_panel` tool alongside `ask_user` and
`list_capabilities`. It would have asked a model to choose an operation and
decide to keep it in one call, and the local models already miss `ask_user` four
times in five (`DECISIONS.md`, 2026-09-11). It also writes without anybody
pressing anything, which the platform has not done anywhere else (D8).

## 8. User interface

The drawer holds the workspaces under the chat, which is the second entry the
shell was built to take (`docs/specs/orchestration.md`, section 7).

A workspace is its panels in a column, each in a card with its title, its
provenance, and a refresh control. The components are the ones the chat already
draws: a table is a table, with the same expansion.

A panel that fails to load says so in its own card. One service being down is
not the workspace being broken.

## 9. Deliberately excluded

- **Dragging panels about.** `harness/quality/ui-primitives.txt` allows controls
  from Material UI, and Material UI has no grid layout; every library that does
  would have to be argued past the guard. The position column is there, the
  editing is not, and that argument can be had on its own.
- **Sharing a workspace.** `docs/requirements.md` U-9 leaves it open, and with a
  stub user there is nobody to share with yet.
- **Editing a panel's arguments in place.** Ask the question again and save the
  answer; two ways to say the same thing is one more than this needs.
- **A panel that is not a call.** Free text, a heading, an image - none of them
  are what this is for, and each is a small feature that would want the layout
  editing that is also excluded.

## 10. Acceptance criteria

- **AC-W-101** A result in the chat can be saved to a workspace, and the
  workspace then lists a panel naming the same service and operation.
- **AC-W-102** Opening a workspace calls each panel's operation and draws the
  answer with the component the panel names.
- **AC-W-103** Refreshing a panel calls its operation again; a row created
  between the two appears in the second answer.
- **AC-W-104** A question asked from the workspace screen is answered there and
  can be saved without returning to the chat.
- **AC-W-105** Workspaces survive a restart of the platform.
- **AC-W-106** A panel whose service is unreachable reports it in its own card,
  and the workspace's other panels still draw.

## 11. Harness work this implies

Nothing yet. `guard-ui` stands as it is, because nothing here reaches past
Material UI - which is also why dragging panels is excluded rather than built.
If the layout editing is taken up later, that guard is the first conversation.
