# Acceptance criteria

Every criterion in `PRODUCT.md` has an identifier (`AC-<layer>-<n>`) and a test
named after it. `make check` runs all of them (through `make acceptance`) after a
full build; CI runs them as the last job.

A test lives beside what it exercises. Only the tests that span processes stand
on their own, because only they belong to no single service.

| Location                      | Layer          | Runs against                                                                                                                              | ID prefix |
| ----------------------------- | -------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| `services/<name>/acceptance/` | Go integration | That service's real object graph behind `httptest`, no network. A separate Go module on purpose, so a test cannot reach into `internal/`. | `AC-B-`   |
| `web/acceptance/`             | Application    | The real `App` from `@app-orchestra/web`, rendered against a scripted API.                                                                | `AC-F-`   |
| `e2e/`                        | Product        | The built service binaries serving `web/dist` on free TCP ports.                                                                          | `AC-E-`   |

## Adding a criterion

1. Add the row to `PRODUCT.md` with the next identifier.
2. If the behaviour is visible through an API, change that service's
   `api/openapi.yaml` first and `make generate`.
3. Write the test in the lowest layer that can prove the criterion. Name the test
   (or its `describe`) with the identifier.
4. `make check`.

## Browser-driven E2E

`make browsers` downloads the pinned Chromium once (`make setup` does it too).
`make acceptance-browser` runs `e2e/browser/*.spec.ts`; on failure a trace is
kept under `e2e/test-results` (uploaded by CI).
