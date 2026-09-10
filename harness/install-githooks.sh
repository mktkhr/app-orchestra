#!/bin/sh
# Points git at the repository owned hooks directory (harness/githooks).
# Runs from `pnpm install` (prepare) and from `make setup`; harmless outside a git checkout.
set -eu
cd "$(dirname "$0")/.."
if [ -d .git ] && command -v git >/dev/null 2>&1; then
  git config core.hooksPath harness/githooks
  echo "git hooks: core.hooksPath=harness/githooks"
fi
