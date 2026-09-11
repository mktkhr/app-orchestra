# app-orchestra — Repository Harness entry points.
#
# These targets are the only interface an agent (or a human) needs. Every
# check is an error, never a warning; if a target fails, fix the code.
# Output is deliberately terse: "ok: <target>" on success, the diagnostics on
# failure (harness/quiet.sh; ORCHESTRA_VERBOSE=1 shows everything).
# See README.md for what each target means and harness/quality/README.md for the policy.

SHELL := /bin/sh
.DEFAULT_GOAL := help
.SILENT:

ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
include $(ROOT)/harness/quality/toolchain.mk

TOOLS_BIN     := $(ROOT)/.tools/bin
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GOLANGCI_CFG  := $(ROOT)/harness/quality/go/golangci.yml
VP            := pnpm exec vp
Q             := sh $(ROOT)/harness/quiet.sh

# A backend service is a directory under services/ with a go.mod. Nothing else
# declares the list: adding a service is adding the directory, and every target
# below picks it up.
SERVICES     := $(patsubst services/%/go.mod,%,$(wildcard services/*/go.mod))
SERVICE_DIRS := $(addprefix services/,$(SERVICES))
SPECS        := $(wildcard $(addsuffix /api/openapi.yaml,$(SERVICE_DIRS)))

# Acceptance suites are separate modules on purpose, so that a test cannot reach
# into the internal/ tree of the service it exercises.
ACCEPTANCE_MODULES := $(patsubst %/go.mod,%,$(wildcard $(addsuffix /acceptance/go.mod,$(SERVICE_DIRS))))

# Deepest first: services/<n>/acceptance must claim its files before
# services/<n> does. harness/githooks/pre-commit relies on the same ordering.
GO_MODULES := $(ACCEPTANCE_MODULES) harness/guard/archcheck $(SERVICE_DIRS)
GO_GEN_MOD := harness/gen

# Everything produced from the specs. `make generate` rewrites these;
# `make guard-generated` fails when they are stale.
GENERATED := $(addsuffix /internal/adapter/openapi/openapi.gen.go,$(SERVICE_DIRS)) \
             $(addsuffix .d.ts,$(addprefix web/src/shared/api/gen/,$(SERVICES)))

.PHONY: help setup tools hooks clean services \
        generate generate-services generate-web api-lint guard-generated guard-generated-ops \
        fmt fmt-check lint test build check acceptance guard \
        services-fmt services-fmt-check services-lint services-test services-build service-run \
        web-fmt web-fmt-check web-lint web-typecheck web-test web-build web-dev \
        guard-arch guard-fsd guard-suppressions guard-filelen guard-ui guard-ignored guard-duplication guard-coverage guard-browser guard-a11y guard-layout guard-protected guard-test \
        acceptance-services acceptance-web acceptance-e2e acceptance-browser browsers

## ---------------------------------------------------------------- overview
help: ## Show this help
	grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

services: ## List the backend services this repository holds
	printf '%s\n' $(SERVICES) | sed '/^$$/d' || true

setup: tools hooks ## One-time setup: pinned tools, dependencies, git hooks, Playwright browser
	$(Q) setup:pnpm-install pnpm install --frozen-lockfile
	$(MAKE) browsers

tools: $(GOLANGCI_LINT) ## Install pinned Go tooling into .tools/bin

# Rebuilt whenever toolchain.mk changes: golangci-lint links the standard
# library of the Go that built it, so a binary built by an older Go reports
# phantom type errors in the newer one's sources.
$(GOLANGCI_LINT): $(ROOT)/harness/quality/toolchain.mk
	mkdir -p $(TOOLS_BIN)
	rm -f $@
	$(Q) tools:golangci-lint env GOBIN=$(TOOLS_BIN) GOTOOLCHAIN=go$(GO_VERSION) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

hooks: ## Point git at the repository owned hooks (harness/githooks/)
	$(Q) hooks sh harness/install-githooks.sh

## ---------------------------------------------------------------- aggregate
fmt: services-fmt web-fmt ## Format all sources in place (the only fix an agent should need before make check)

fmt-check: services-fmt-check web-fmt-check ## Verify formatting without writing

lint: api-lint services-lint web-lint guard ## Every static check: contract lint, Go lint, web lint + typecheck, guards

test: services-test web-test guard-test guard-coverage ## Unit tests of the services, the web app and the guards

build: services-build web-build ## Production build of every service and the web app

check: fmt-check lint test build acceptance ## Every quality gate in one target: format, contract, lint, typecheck, guards, tests, build, acceptance (incl. browser). This is what an agent runs; pre-push and CI run the same.

acceptance: build acceptance-services acceptance-web acceptance-e2e acceptance-browser guard-browser ## Executable acceptance criteria (integration, e2e, browser); part of make check

guard: guard-generated guard-generated-ops guard-arch guard-fsd guard-suppressions guard-filelen guard-ui guard-ignored guard-duplication ## Contract freshness, architecture, suppression, file-length and design-system guards

guard-browser: guard-a11y guard-layout ## Browser-driven quality gates (needs make build, make browsers)

## ---------------------------------------------------------------- contract (OpenAPI first)
generate: generate-services generate-web ## Regenerate server and client code from every services/*/api/openapi.yaml

generate-services: ## oapi-codegen: Go models + strict server interface + embedded spec, per service
	for s in $(SERVICES); do \
	  [ -f "services/$$s/api/openapi.yaml" ] || continue; \
	  mkdir -p "services/$$s/internal/adapter/openapi"; \
	  $(Q) generate-services:$$s go -C $(GO_GEN_MOD) tool oapi-codegen \
	    -config ../../harness/gen/oapi-codegen.yaml \
	    -o ../../services/$$s/internal/adapter/openapi/openapi.gen.go \
	    ../../services/$$s/api/openapi.yaml || exit 1; \
	done

generate-web: ## openapi-typescript: one module of types per service
	for s in $(SERVICES); do \
	  [ -f "services/$$s/api/openapi.yaml" ] || continue; \
	  mkdir -p web/src/shared/api/gen; \
	  $(Q) generate-web:$$s pnpm exec openapi-typescript "services/$$s/api/openapi.yaml" \
	    -o "web/src/shared/api/gen/$$s.d.ts" --immutable --export-type || exit 1; \
	done

api-lint: ## Lint every services/*/api/openapi.yaml with Redocly (recommended-strict + repository rules)
	if [ -z "$(SPECS)" ]; then echo "api-lint: no specs yet"; else $(Q) api-lint pnpm exec redocly lint $(SPECS); fi

guard-generated: generate ## Fail when generated code does not match the specs
	$(Q) guard-generated sh -c 'git diff --exit-code -- $(GENERATED) || { echo "generated code is stale: run make generate and commit the result"; exit 1; }'

guard-generated-ops: ## Fail when a generated artifact is missing an operationId the spec declares (e.g. oapi-codegen silently dropping a 3.2 `query` operation)
	$(Q) guard-generated-ops sh harness/guard/generated-ops.sh

## ---------------------------------------------------------------- services (Go)
services-fmt: $(GOLANGCI_LINT) ## gofmt / goimports / gci in place, every Go module
	for m in $(GO_MODULES); do $(Q) services-fmt:$$m sh -c "cd $$m && $(GOLANGCI_LINT) fmt --config $(GOLANGCI_CFG) ./..." || exit 1; done

services-fmt-check: $(GOLANGCI_LINT) ## Fail when Go sources are not formatted
	for m in $(GO_MODULES); do $(Q) services-fmt-check:$$m sh -c "cd $$m && $(GOLANGCI_LINT) fmt --config $(GOLANGCI_CFG) --diff ./..." || exit 1; done

services-lint: $(GOLANGCI_LINT) ## go vet + golangci-lint (staticcheck, revive, gosec, ...), every Go module
	for m in $(GO_MODULES); do \
	  [ -n "$$(cd $$m && go list ./... 2>/dev/null)" ] || continue; \
	  $(Q) services-lint:vet:$$m sh -c "cd $$m && go vet ./..." || exit 1; \
	  $(Q) services-lint:golangci:$$m sh -c "cd $$m && $(GOLANGCI_LINT) run --config $(GOLANGCI_CFG) ./..." || exit 1; \
	done

services-test: ## go test with race detector, every service
	for s in $(SERVICES); do \
	  [ -n "$$(cd services/$$s && go list ./... 2>/dev/null)" ] || continue; \
	  $(Q) services-test:$$s sh -c "cd services/$$s && go test -race -count=1 ./..." || exit 1; \
	done

services-build: ## Compile every service into services/<name>/bin/api
	for s in $(SERVICES); do \
	  [ -d "services/$$s/cmd/api" ] || continue; \
	  $(Q) services-build:$$s sh -c "cd services/$$s && CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api" || exit 1; \
	done

service-run: services-build ## Run one service locally: make service-run SERVICE=platform (not quiet: it is a server)
	@:
	if [ -z "$(SERVICE)" ]; then echo "service-run: set SERVICE=<name>; known: $(SERVICES)"; exit 1; fi
	./services/$(SERVICE)/bin/api

## ---------------------------------------------------------------- web
web-fmt: ## oxfmt in place (all workspace packages)
	$(Q) web-fmt $(VP) fmt

web-fmt-check: ## Fail when TS/JS/JSON/CSS/MD sources are not formatted
	$(Q) web-fmt-check $(VP) fmt --check

web-lint: ## oxlint (type aware) + TypeScript type check, warnings are errors
	$(Q) web-lint $(VP) check --no-fmt

web-typecheck: ## TypeScript type check only
	$(Q) web-typecheck $(VP) check --no-fmt --no-lint

web-test: ## Vitest unit tests of the web package
	$(Q) web-test $(VP) -C web test run

web-build: ## Production build into web/dist
	$(Q) web-build $(VP) -C web build

web-dev: ## Vite dev server, proxies /api to :8080 (not quiet: it is a server)
	$(VP) -C web dev

## ---------------------------------------------------------------- guards
guard-arch: ## Service layer dependency check, every service (harness/quality/architecture.json)
	$(Q) guard-arch go -C harness/guard/archcheck run .

guard-fsd: ## Web Feature-Sliced Design import check (harness/quality/architecture.json)
	$(Q) guard-fsd node harness/guard/fsd.ts

guard-suppressions: ## Every lint suppression must be registered in harness/quality/suppressions.allow
	$(Q) guard-suppressions sh harness/guard/suppressions.sh

guard-filelen: ## Fail when a hand-written source file is longer than harness/quality/file-length.txt allows
	$(Q) guard-filelen sh harness/guard/filelen.sh

guard-duplication: ## Fail when two web functions share a structure (harness/quality/duplication.txt)
	$(Q) guard-duplication node harness/guard/duplication.ts

guard-coverage: ## Fail when a Go package is below its minimum in harness/quality/coverage.txt
	$(Q) guard-coverage sh harness/guard/coverage.sh

guard-ignored: ## Fail when a protected path is git-ignored
	$(Q) guard-ignored sh harness/guard/ignored-paths.sh

guard-ui: ## Fail when a raw control is used outside the shared UI layer (harness/quality/ui-primitives.txt)
	$(Q) guard-ui node harness/guard/ui-primitives.ts

guard-a11y: ## WCAG 2.2 AA audit of the built product, zero violations
	$(Q) guard-a11y pnpm exec playwright test --config harness/quality/browser/playwright.config.ts a11y.spec.ts

guard-layout: ## Measured layout invariants: target size, visible boundaries, no sideways scroll
	$(Q) guard-layout pnpm exec playwright test --config harness/quality/browser/playwright.config.ts layout.spec.ts

guard-protected: ## Fail when harness files changed relative to origin/main (CI: pull requests)
	$(Q) guard-protected sh harness/guard/protected-paths.sh

guard-test: ## Unit tests of the guards themselves
	$(Q) guard-test:archcheck go -C harness/guard/archcheck test -count=1 ./...
	$(Q) guard-test:fsd $(VP) test run

## ---------------------------------------------------------------- acceptance
acceptance-services: ## In-process HTTP integration tests, every services/<name>/acceptance
	for m in $(ACCEPTANCE_MODULES); do \
	  [ -n "$$(cd $$m && go list ./... 2>/dev/null)" ] || continue; \
	  $(Q) acceptance-services:$$m sh -c "cd $$m && go test -race -count=1 ./..." || exit 1; \
	done

acceptance-web: ## Application-level web tests (web/acceptance)
	$(Q) acceptance-web $(VP) -C web/acceptance test run

acceptance-e2e: ## Process-level end-to-end tests against the built product (needs make build)
	$(Q) acceptance-e2e $(VP) -C e2e test run

acceptance-browser: ## Playwright tests in headless Chromium against the built product (needs make build, make browsers)
	$(Q) acceptance-browser pnpm -C e2e exec playwright test

browsers: ## Download the Chromium build Playwright is pinned to
	$(Q) browsers pnpm -C e2e exec playwright install chromium

## ---------------------------------------------------------------- misc
clean: ## Remove build output
	$(Q) clean rm -rf $(addsuffix /bin,$(SERVICE_DIRS)) web/dist
