# PRODUCT.md — app-orchestra

**The product specification is not written yet.** This file records what has
been decided so far and points at where the thinking lives. An agent working
in this repository today is working on the harness, not on the product.

The document is in English because the repository is. The product's own user
interface will be in Japanese.

## 1. Purpose

app-orchestra is a proof of concept for operating a set of microservices
through natural language.

A user asks a question in plain language. An LLM decides which service and
which API answers it, calls it, and the result is rendered as an interface
rather than as prose: a list endpoint becomes a paginated table, a single
entity becomes a detail card, a write endpoint becomes a form with a submit
button the user presses. The point is that a user reaches many services
without visiting any of them.

## 2. What has been decided

|        | Decision                                                                                                                                                                                                                                                        |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **D1** | The system decomposes into seven subprojects. The first vertical slice is: dummy services, service catalogue, LLM orchestration, dynamic UI, chat screen. Authentication and workspaces come after it.                                                          |
| **D2** | The service catalogue is sent whole on every request and kept warm in the prompt cache. Staged narrowing (service, then genre, then API) is not adopted for cost reasons: at the expected scale it is several times more expensive and three times the latency. |
| **D3** | Component selection is deterministic, derived from the OpenAPI response schema plus `x-ui-hint`. No LLM call participates in rendering.                                                                                                                         |
| **D4** | The frontend is a Vite + React Router SPA with Material UI and Toolpad Core. Orchestration lives in the Go backend, not in a Node BFF.                                                                                                                          |

The consequence of D2 and D3 together is that the LLM has exactly one job:
choose the API and fill its parameters. Everything downstream of that is
ordinary code.

## 3. Scope of the first vertical slice

In scope: two dummy services with three endpoints each (list, detail, create);
OpenAPI ingestion into a catalogue; natural language to one API call with
parameters; three rendering components (table, detail card, form); a chat
screen.

Out of scope for the first slice: authentication and authorisation (a stub
user, with the seat for the permission filter left open in the catalogue),
workspaces, multi-turn context, and the genre layer between service and API.

## 4. How the slice is built

`docs/specs/orchestration.md` is the design: the ports, the data flow, the
platform's contract, the rendering rule, and how a catalogue becomes tool
definitions. It also records what was deliberately left out and why.

## 5. Acceptance criteria

Every criterion has a test named after it, beside what it exercises, per
`docs/acceptance.md`.

### Platform (`AC-B-*`)

- **AC-B-101** The platform builds a catalogue from two running services and
  produces one tool definition per operation.
- **AC-B-102** A question that maps to a list endpoint returns `kind: "result"`
  with `component: "table"`, having called the service.
- **AC-B-103** A question that maps to a create endpoint returns `kind: "form"`
  and does not call the service.
- **AC-B-104** `POST /api/invoke` executes the create and returns the created
  entity.
- **AC-B-105** A question naming a status outside the enum returns `kind: "ask"`
  with the candidate values and their Japanese labels.
- **AC-B-106** A question matching no endpoint returns `kind: "none"`.

### Web (`AC-F-*`)

- **AC-F-101** `component: "table"` renders a paginated MUI table.
- **AC-F-102** `component: "form"` renders inputs derived from the schema, with
  the model's values prefilled, and a submit button that posts to
  `/api/invoke`.
- **AC-F-103** `component: "choice"` renders the options; picking one asks
  again.
- **AC-F-104** The empty conversation shows example questions covering a list,
  a create, and a status the enum does not have.
- **AC-F-105** A rendered table can be expanded into a full-screen modal
  showing the same rows.
- **AC-F-106** Every result shows the service and operation id that produced
  it, and the arguments the model chose can be expanded.

### End to end (`AC-E-*`)

- **AC-E-101** The built product answers a question end to end against both
  running dummy services.

## 6. Definition of done

Unchanged from the harness: `make check` is green and every acceptance
criterion in this file has a passing test.
