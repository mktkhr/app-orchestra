#!/bin/sh
# post-edit.sh - Claude Code PostToolUse hook: speak only when the file that was
# just edited is wrong.
#
# This is the one runtime-specific piece of the Repository Harness, and nothing
# depends on it. Everything it reports is reported again by harness/githooks/pre-commit, by
# `make check` and by CI. What it buys is timing: a commit is dozens of edits
# away, and an agent that learns at commit time has already built on top of the
# mistake. Reporting now costs one line; reporting later costs the work in
# between.
#
# Silence is the success case. There is no "ok:" line here, unlike
# harness/quiet.sh - a hook that speaks on success would put a message in the
# model's context after every single edit, which is the opposite of the point.
#
# Exit 0 says nothing. Exit 2 hands stderr back to the model.
set -u

root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$root" 2>/dev/null || exit 0

# The hook payload arrives on stdin. Edit and Write both carry the path.
file=$(jq -r '.tool_input.file_path // .tool_response.filePath // empty' 2>/dev/null) || exit 0
[ -n "$file" ] || exit 0

case $file in
  "$root"/*) rel=${file#"$root"/} ;;
  /*)        exit 0 ;;   # outside the repository; not this hook's business
  *)         rel=$file ;;
esac
[ -f "$rel" ] || exit 0

problems=""

# 1. The harness is not the agent's to change (AGENTS.md, principle 2). The
#    pre-commit hook refuses the commit; this says so while it can still be
#    undone cheaply.
if [ -f harness/quality/protected-paths.txt ]; then
  while IFS= read -r protected; do
    case $protected in ''|\#*) continue ;; esac
    case $rel in
      "$protected"|"$protected"/*)
        problems="${problems}${rel} is part of the Repository Harness (listed in harness/quality/protected-paths.txt).
  Changing it is a separate, acknowledged change: explain it in DECISIONS.md and
  commit with ORCHESTRA_ALLOW_HARNESS_CHANGE=1. If you edited it to make a check
  pass, revert it and fix the code instead.
"
        break
        ;;
    esac
  done < harness/quality/protected-paths.txt
fi

# 2. Formatting. Every case this reports is fixed by `make fmt`.
case $rel in
  *.go)
    if command -v gofmt >/dev/null 2>&1 && [ -n "$(gofmt -l "$rel" 2>/dev/null)" ]; then
      problems="${problems}${rel} is not gofmt clean. Run: make fmt
"
    fi
    ;;
  *.ts|*.tsx|*.js|*.mjs|*.json|*.css|*.md|*.yml|*.yaml)
    if [ -x node_modules/.bin/vp ] && ! node_modules/.bin/vp fmt --check "$rel" >/dev/null 2>&1; then
      problems="${problems}${rel} is not formatted. Run: make fmt
"
    fi
    ;;
esac

[ -n "$problems" ] || exit 0

printf '%s' "$problems" >&2
exit 2
