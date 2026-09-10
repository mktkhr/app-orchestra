#!/bin/sh
# Harness protection guard.
#
# The Repository Harness (quality policies, guards, hooks, CI, Makefile) must
# not change as a side effect of product work. This script fails when any
# protected path differs from the base revision, unless the change is
# explicitly acknowledged:
#
#   locally  ORCHESTRA_ALLOW_HARNESS_CHANGE=1 git commit ...
#   in CI    the pull request carries the "harness" label (see .github/workflows/ci.yml)
#
# Usage:
#   protected-paths.sh --staged          compare the index with HEAD (pre-commit)
#   protected-paths.sh [<base-ref>]      compare the working tree with a ref (CI; default origin/main)
set -eu
cd "$(dirname "$0")/../.."

protected_list="harness/quality/protected-paths.txt"

if [ "${ORCHESTRA_ALLOW_HARNESS_CHANGE:-0}" = "1" ]; then
  echo "protected-paths: change explicitly allowed (ORCHESTRA_ALLOW_HARNESS_CHANGE=1)"
  exit 0
fi

paths=$(grep -vE '^[[:space:]]*(#|$)' "$protected_list")

if [ "${1:-}" = "--staged" ]; then
  # A fresh repository has no HEAD yet; nothing to compare against.
  if ! git rev-parse --verify -q HEAD >/dev/null; then
    echo "protected-paths: no HEAD yet, skipping"
    exit 0
  fi
  changed=$(git diff --cached --name-only -- $paths)
else
  base=${1:-origin/main}
  if ! git rev-parse --verify -q "$base" >/dev/null; then
    echo "protected-paths: base ref $base not found, skipping"
    exit 0
  fi
  changed=$(git diff --name-only "$base"...HEAD -- $paths 2>/dev/null || git diff --name-only "$base" -- $paths)
fi

if [ -z "$changed" ]; then
  echo "protected-paths: harness untouched"
  exit 0
fi

echo "protected-paths: the following Repository Harness files changed:"
printf '%s\n' "$changed" | sed 's/^/  /'
echo
echo "Quality settings, guards, hooks and CI are not part of product work."
echo "Fix the code instead of the check. If the harness itself needs to change,"
echo "make it a separate, explicitly acknowledged change:"
echo "  ORCHESTRA_ALLOW_HARNESS_CHANGE=1 git commit ...   (local)"
echo "  add the 'harness' label to the pull request       (CI)"
exit 1
