# Authentication and authorisation

The third subproject. `docs/specs/orchestration.md` and `docs/specs/workspaces.md`
come first; this one fills seats both of them left open.

## 1. What it proves

That the catalogue was the right place to decide what a person can reach. Two
seats were left for this - the permission filter in the catalogue
(`TODO(auth)` in `Orchestrator.Invoke`) and the owner of a workspace
(`stubOwner`) - and filling them should change those two lines and very little
else.

It also proves the model cannot be talked into anything. An operation a person
may not call is not in the tools the model is offered, and would be refused at
the mouth if it somehow arrived (FR-A-6, FR-A-7).

## 2. Decisions taken here

|        | Decision                                                                                                                                       |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| **A1** | Identity sits behind an `Authenticator` port. The implementation here reads local accounts from the platform's own database.                   |
| **A2** | A session is an opaque token in an HttpOnly cookie, stored alongside its user. No token is ever readable by script.                            |
| **A3** | Permission is per operation, not per service (`docs/requirements.md` U-6, settled here).                                                       |
| **A4** | The catalogue is filtered once per request, and everything downstream reads the filtered one. The tool list and `/api/invoke` cannot disagree. |
| **A5** | Two roles: `admin` and `user`. An admin may read every account and set anybody's permissions; that is the whole of what the role buys.         |
| **A6** | The user reaches the usecase as an argument, not through the context. A permission check that can be forgotten is one that will be.            |
| **A7** | A workspace belongs to the person who made it, and only they see it. Sharing stays out (`docs/requirements.md` U-9).                           |

## 3. Identity

```
User
  id        assigned by the platform
  name      what they type to sign in
  role      admin | user
  hash      argon2id of their password
```

The `Authenticator` port takes a name and a password and answers with a user or
nothing. `adapter/auth/local` implements it over the same SQLite file the
workspaces live in. An OIDC adapter would implement the same port, which is the
abstraction layer `docs/requirements.md` section 5 asks for - built as a seam
rather than as a second implementation nobody has asked to run yet.

A session token is random, opaque, and stored with the user it belongs to and
the time it expires. It travels in an HttpOnly, SameSite cookie: a token a
script can read is a token a script can leak, and the browser has no use for
its own session token.

The first admin is seeded when the database is created, from
`ORCHESTRA_ADMIN_PASSWORD`. An empty one stops the platform, for the same
reason an unset database path does: a default password is a way of having no
password at all while appearing to have one.

## 4. Permission

```
Permission
  user_id
  service       "inventory"
  operation_id  "ListInventoryItems"
```

A row says this person may call this operation. No row says they may not; there
is no deny, because there is nothing to override.

Per operation rather than per service (A3). `docs/requirements.md` FR-A-4 speaks
of access to a microservice, and that is the shape an administrator thinks in -
but "may look at stock, may not create it" is a sentence this product has to be
able to say, and a service-level grant cannot say it. The administration screen
groups the rows by service so that granting a whole service stays one gesture.

An admin holds every permission implicitly. Seeding rows for them would mean
seeding more every time a service appears.

## 5. Where the check happens

Once, where the catalogue is built for a request:

```
domain.Catalog                  every exposed operation
  └─ Catalog.For(permissions)   the ones this person may call
       ├─ ToolsFor              what the model is offered
       └─ Catalog.Find          what /api/invoke will run
```

The model is never told about an operation the person cannot call (FR-A-6), and
a call that arrives anyway finds nothing in the catalogue and is refused
(FR-A-7). This is the same argument `x-orchestra-expose` was filtered at the
catalogue for (D13): a rule applied in two places is a rule that will disagree
with itself.

`Orchestrator.Plan` and `Orchestrator.Invoke` take the user. Not the context: a
value in a context is one a handler can forget to put there, and the failure is
a permission check that silently passes. A parameter cannot be forgotten,
because the code does not compile without it.

## 6. Contract

```
POST   /api/session               { name, password } -> { id, name, role }
DELETE /api/session               sign out
GET    /api/session               -> the signed-in user, or 401

GET    /api/users                 admin only -> [{ id, name, role }]
GET    /api/users/{id}/permissions admin only -> [{ service, operationId }]
PUT    /api/users/{id}/permissions admin only  { permissions: [...] }
```

Everything else gains a 401 when nobody is signed in. `GET /api/health` does
not: it answers whether the process is up, which is not a question about a
person.

## 7. What changes elsewhere

- `Workspaces` drops `stubOwner` and takes the user. A workspace lists, reads
  and writes as its owner, and one belonging to somebody else is not found
  rather than forbidden - a 403 tells you a thing exists.
- A panel naming an operation the person may no longer call fails in its own
  card, which is the path AC-W-106 already built.
- The web shell shows who is signed in and offers to sign out. The users screen
  appears in the drawer for an admin and not for anybody else - and the
  endpoints refuse it either way, because a hidden control is not a check.

## 8. Deliberately excluded

- **A real identity provider.** The port is the requirement
  (`docs/requirements.md` section 5); running Keycloak would test Keycloak.
- **Sharing a workspace** (U-9), still.
- **Anything an admin does besides reading accounts and setting permissions.**
  No creating accounts through the UI or the API, no resetting passwords, no
  deleting people. Each is a small screen (or route) and none of them is
  what this proves. `pkg/app.Config.SeedAccounts` puts a non-admin account
  in place before a process ever serves a request - the same moment
  `ORCHESTRA_ADMIN_PASSWORD` seeds the first admin, and, since
  `docs/plans/auth.md` Task 6, reachable from a built binary too via
  `ORCHESTRA_SEED_ACCOUNTS` (`internal/infra/config`) - which is not a
  second, softer version of the thing excluded here: nothing over HTTP
  creates an account either way (`DECISIONS.md`, 2026-09-12).
- **Auditing who called what.** `docs/requirements.md` section 7 puts it out of
  scope for the PoC, and it is a feature of its own.

## 9. Acceptance criteria

- **AC-A-101** Signing in with a correct password returns the user; a wrong one
  returns 401 and no session.
- **AC-A-102** Every endpoint but `GET /api/health` answers 401 without a
  session.
- **AC-A-103** A question from a person who may call only one service is
  answered from that service, and the tools the planner was offered name no
  operation from the other.
- **AC-A-104** `POST /api/invoke` naming an operation the person may not call
  returns the same status as one that does not exist, and calls nothing.
- **AC-A-105** A workspace made by one person is not listed or readable by
  another.
- **AC-A-106** An admin can read the accounts and set another person's
  permissions; a non-admin gets 403 from the same endpoints.
- **AC-A-107** A session survives a reload and ends on sign-out.

## 10. Harness work this implies

None expected. `make check` gains no dependency: the accounts are rows in the
file the workspaces already use, and every test seeds its own.
