# CLAUDE.md

The rules for working here are in `AGENTS.md`. It is the document every agent
runtime reads, and it is imported below rather than copied, so the two can never
drift apart:

@AGENTS.md

## What Claude Code adds

`.claude/settings.json` configures two things. **Nothing in the repository
depends on either of them** - every check they make is made again by
`harness/githooks/pre-commit`, by `make check` and by CI. They exist only to make the same
answer arrive sooner.

**A permission allowlist.** `make`, the read-only `go` and `git` subcommands and
the ordinary file-reading shell commands run without a prompt, because they are
the whole command surface this repository asks for. `git commit --no-verify` and
`git push --no-verify` are denied outright: `AGENTS.md` already says they are not
fixes, and this is the point where that stops being advice.

**A PostToolUse hook**, `harness/claude/post-edit.sh`. After each Edit or Write
it looks at the one file that changed and says nothing unless that file is
unformatted, or belongs to the harness. Silence is the normal outcome. A message
means the edit wants `make fmt`, or wants undoing.

Both are protected paths, like the rest of the harness.
