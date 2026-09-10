# Pinned versions of tools that are not managed by go.mod or pnpm.
# Part of the Repository Harness. Bump deliberately, in a dedicated change.
GOLANGCI_LINT_VERSION := v2.13.2
# Patch level included: GOTOOLCHAIN needs the full version, and golangci-lint
# must be built by the same Go it analyses (see DECISIONS.md, 2026-09-10).
GO_VERSION            := 1.27.1
NODE_VERSION          := 24
PNPM_VERSION          := 12.3.4
# Code generators (pinned in harness/gen/go.mod and pnpm-workspace.yaml; listed for reference).
OAPI_CODEGEN_VERSION  := v2.8.0
OPENAPI_TS_VERSION    := 7.13.0
