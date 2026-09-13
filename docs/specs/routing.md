# Addresses

The eighth subproject. It changes no feature: every screen this product has
is already reachable. What changes is what a screen's address looks like,
and whether one survives being pasted to somebody else.

## 1. What it proves

That a decision nobody argued is a decision worth re-opening.

`web/src/app/model/useHashRoute.ts` reads the route from
`window.location.hash`, and its doc comment is a careful argument - against
adopting `react-router`. That is not the same question. Whether to take a
router library and whether to put the route in the hash are two decisions,
and only the first one was ever written down. The second was made silently
alongside it.

The real reason for the hash is in a different file.
`internal/infra/httpserver/router.go` serves the built frontend with

```go
root.Handle("/", http.FileServer(http.Dir(staticDir)))
```

which answers 404 to any path that is not a file on disk. So
`/workspaces/abc` works while the browser is doing the navigating and breaks
the moment somebody reloads, and the hash is what avoids that. It is a real
constraint and it was never the argument given - and it is a constraint this
repository owns, in a server it wrote, three lines long.

## 2. Decisions taken here

|        | Decision                                                                                                                                                                                                           |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **R1** | A screen's address is a path. `/`, `/workspaces/{id}`, `/users` - not `#workspace-{id}`. An address a person can read, paste and bookmark is the thing a URL is for.                                               |
| **R2** | The platform serves `index.html` for any request that is not an API route and is not a file it has. A missing asset still answers 404: a request for a `.js` that does not exist must not be handed HTML to parse. |
| **R3** | `react-router` does the routing. The alternative is the History API by hand, which is thirty lines until the first redirect, the first nested route, and the first time a route needs data before it renders.      |
| **R4** | The hash addresses keep working, by redirecting once on arrival. Somebody has those links open right now; a URL that stops working is the thing R1 exists to prevent, and it would be odd to start by doing it.    |
| **R5** | Signing in is not a route. `AuthGate` decides whether anybody sees a screen at all, and it already does; making the sign-in screen an address would mean deciding what to do with the address somebody came for.   |

## 3. The addresses

```
/                    the chat
/workspaces/{id}     one workspace
/users               the admin's accounts screen
```

`/users` is a screen only an admin reaches. It stays a route rather than a
condition: the endpoints behind it already refuse a non-admin
(`docs/specs/auth.md` section 7), and a screen that answers "you may not see
this" is better than a link that silently does nothing.

An address that matches nothing renders the chat, which is what the hash
router does today.

## 4. The server's part

```
/api/...        the API, as now
/<a file>       the file, as now
/<anything>     index.html
/<a missing asset>   404
```

The last two lines are the whole difficulty, and they are not the same rule.
A person typing `/workspaces/abc` wants the application; a browser fetching
`/assets/index-abc123.js` that is not there wants an error. Handing HTML to
the second produces a syntax error from a file the developer can see exists
in the listing, which is a bad hour.

The rule: a request whose path has a file extension is a request for a file,
and answers 404 when the file is missing. Everything else is the
application. It is a heuristic, and it is the one every static host uses,
and it fails only for an extensionless asset - which this build does not
produce and a guard can say so if one ever does.

## 5. What does not change

- **The conversation's key** (`docs/specs/context.md` M6). One per screen:
  `"chat"`, and one per workspace id. The id comes from the route rather
  than the hash, and nothing else moves.
- **`AuthGate`** (R5).
- **Every API path.** No contract changes in this subproject at all.

## 6. Deliberately excluded

- **Server-side rendering.** The platform serves one HTML file and the
  application draws itself. Nothing here asks for more.
- **A route per panel.** A panel is a card in a workspace, not a place.
- **Loader-based data fetching.** `react-router` can fetch before a route
  renders; every screen here already fetches in its own component, and
  moving that is a rewrite with no complaint behind it.

## 7. Acceptance criteria

- **AC-R-101** `/workspaces/{id}` typed into a fresh browser, or reloaded,
  draws that workspace. Same for `/users`.
- **AC-R-102** A missing asset answers 404, not HTML.
- **AC-R-103** An old `#workspace-{id}` address arrives at
  `/workspaces/{id}`, once, without a second entry in the history.
- **AC-R-104** `GET /api/...` is untouched: no API path is shadowed by the
  fallback, including one that does not exist, which must still answer as
  the API and not as the application.
- **AC-R-105** A person with no session still reaches the sign-in screen
  from any address, and lands where they were going after signing in.

## 8. Harness work this implies

`harness/quality/browser/screens.ts` navigates to `/#users` and
`/#workspace-{id}`; both become paths. It is a protected path - a change
there is argued in `DECISIONS.md` and committed with
`ORCHESTRA_ALLOW_HARNESS_CHANGE=1`.

`e2e/browser/*.spec.ts` navigate by hash too, and `e2e/src/*.test.ts` do not
navigate at all, so only the browser suites move.
