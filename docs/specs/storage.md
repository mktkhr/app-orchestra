# One file, one way in

The eleventh subproject. It adds nothing a person can see, and removes a
class of failure they have already met three times in a day.

## 1. What it proves

That "database is locked" was never about the machine being busy.

`make guard-layout` failed three times in one day with a 500, and each time
it was attributed to load — a reading recorded in `DECISIONS.md` on the
strength of eight clean runs on a quiet machine. That reading was wrong, and
the way it was wrong is worth keeping: eight clean runs measured a machine
that was not being asked to do the thing that breaks.

Asked directly — forty concurrent workspace creations against a platform
started exactly the way the browser gates start one — it fails about ten
percent of the time, quietly:

```
{"message":"resolving session: looking up session: database is locked (5) (SQLITE_BUSY)"}
```

Not in the write. In `requireSession`, reading the session row that every
request reads before it does anything at all.

## 2. What is actually wrong

Three things, each of which alone would be survivable.

**The process opens the file four times.** `openDB` is called by `Store`,
`Users`, `Sessions` and `Permissions`, and each gets its own `sql.DB` with
its own `SetMaxOpenConns(1)`. The comment on that constant says one
connection "avoids 'database is locked' errors from two goroutines racing to
open the file". It would, if there were one. There are four, and a read on
one collides with a write on another.

**Nothing waits.** No `busy_timeout` is set, so SQLite does not block on a
held lock for even a millisecond — it returns `SQLITE_BUSY` immediately. The
default is zero, and zero is what this has.

**Nothing shares.** With no `journal_mode`, the file is in `delete` mode,
where a writer excludes every reader. WAL exists precisely so that one
writer and many readers can proceed at once, and it is one pragma.

## 3. Decisions taken here

|        | Decision                                                                                                                                                                                                          |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **S1** | One `sql.DB` for the file, opened once and shared by every store. "One connection" has to mean one, or it means nothing.                                                                                          |
| **S2** | WAL, so a reader and a writer are not each other's problem. A session lookup on every request is the reader this exists for.                                                                                      |
| **S3** | A busy timeout, so a lock that is held for a moment is waited for rather than reported as a failure. Five seconds: long enough that no honest contention reaches a person, short enough to be a bug when it does. |
| **S4** | A 500 says what happened in the log. The failure above was returned to a browser and written nowhere - three separate investigations read "500" and guessed.                                                      |

## 4. What does not change

- **The schema**, its migrations, and every store's own methods.
- **`ORCHESTRA_DB_PATH`**, and the fact that the accounts live in the file
  the workspaces already use (`docs/specs/auth.md` section 10).
- **The single-process assumption.** WAL allows several processes; nothing
  here needs that, and nothing here forbids it either.

## 5. Deliberately excluded

- **A connection pool wider than one.** WAL plus a busy timeout is what this
  needed; a pool is the next thing to try if measurement says so, and
  measurement has not.
- **Any other database.** The argument for SQLite (`docs/specs/workspaces.md`
  W3) is unchanged: a person assembled these rows and nothing else here needs
  a server.
- **Retrying a failed request.** A retry on top of a lock nobody waited for
  is two mechanisms where one pragma will do.

## 6. Acceptance criteria

- **AC-S-101** Forty concurrent requests that each read a session and write a
  row all succeed. This is the measurement in section 1, as a test.
- **AC-S-102** The process opens one `sql.DB` for its database file, however
  many stores read it.
- **AC-S-103** A 500 from any handler is written to the log with the error
  that caused it.
- **AC-S-104** Everything the existing suites assert about workspaces,
  panels, sessions, accounts and permissions still holds - this changes how
  the file is opened, not what is in it.

## 7. Harness work this implies

None expected, and one thing to watch: `make guard-layout`'s intermittent
500 is the symptom this removes. If it still flakes afterwards, the load
explanation gets a second chance and this spec's section 1 was only half the
story - so re-run it enough times to say, and record the number.
