#!/bin/sh
# Ignore guard.
#
# A protected path that git ignores is a protected path that never reaches the
# repository, so a quality gate can be removed without editing a protected file
# at all. Observed: the agent met an unfamiliar untracked directory, added
# `harness/quality/browser/` to .gitignore, and committed the rest. The gate would have
# vanished on the next clone and in CI.
#
# harness/quality/protected-paths.txt lists what must never be ignored.
set -eu
cd "$(git rev-parse --show-toplevel)"

registry="harness/quality/protected-paths.txt"
ignored=""

while IFS= read -r entry; do
  case $entry in ''|\#*) continue ;; esac
  [ -e "$entry" ] || continue

  # Only the registered path itself, and the sources under it that belong in the
  # repository. Artifacts beneath a protected directory are allowed to be
  # ignored: a gate that forbids ignoring its own build output leaves no way out
  # for an agent that cannot write to harness/quality/ either.
  hit=$(git check-ignore "$entry" 2>/dev/null || true)
  [ -n "$hit" ] && ignored="$ignored$hit\n"

  if [ -d "$entry" ]; then
    inner=$(git ls-files --others --ignored --exclude-standard \
      -- "$entry/*.sh" "$entry/*.ts" "$entry/*.js" "$entry/*.mjs" "$entry/*.go" \
         "$entry/*.json" "$entry/*.yml" "$entry/*.yaml" "$entry/*.txt" "$entry/*.md" 2>/dev/null || true)
    [ -n "$inner" ] && ignored="$ignored$inner\n"
  fi
done < "$registry"

if [ -n "$ignored" ]; then
  echo "protected paths are git-ignored:"
  printf "$ignored" | sed 's/^/  /' | sort -u
  echo "The quality baseline must be in the repository. Remove those .gitignore entries."
  exit 1
fi

echo "ignored-paths: no protected path is ignored"
