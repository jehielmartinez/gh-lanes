# AGENTS.md

`lanes` is a terminal kanban board for the open pull requests you authored. It runs as `gh lanes` or as a standalone `lanes` binary. It is written in Go with Bubble Tea and `go-gh`. The product spec is [docs/spec.md](docs/spec.md).

## Agent skills

The dev loop (areas, gates, stop rules, seams) is in [docs/agents/dev-loop.md](docs/agents/dev-loop.md). Read it, and both lists below, before planning a change.

## Non-negotiables

- **No credentials on disk.** The config and state files never hold a token. Auth stays in `gh`'s own storage, read through `go-gh`.
- **PR content is untrusted input.**
  - Strip terminal escape and control sequences from titles, bodies, comments and review text before rendering.
  - Open only `http` and `https` URLs.
  - Pass a URL to the OS opener (`open` on macOS, `xdg-open` on Linux) as a single argument. Never pass it through a shell.
- **Tags are local.** Tags, lane assignments, archive state and snapshots never reach GitHub. No label, project or comment is written to represent them.
- **PRs are keyed by node ID.** Assignments, the archived list and snapshots are keyed by node ID, never by `owner/repo#number`, so renames and transfers keep their tag. Assignments reference the tag ID, never the tag name. The owner and repository filter is the one exception: it is keyed by name ([ADR 0001](docs/adr/0001-filter-keyed-by-name.md)).
- **Each PR is in exactly one lane.** Untagged is always first, and it can't be renamed or deleted. Deleting a tag moves its PRs to Untagged.
- **Local writes are atomic.** Write a temp file in the same directory, then rename it. Both files carry a schema `version`.
- **Config location.** Use `$XDG_CONFIG_HOME/lanes`, falling back to `~/.config/lanes` on every OS, macOS included. `--config` overrides it.
- **A failed refresh keeps the last good data.** It stays on screen marked stale, and the error goes to the status bar.
- **Actions are offered only when GitHub allows them.** Gate them on `viewerCanUpdate`, the repo's merge settings and the PR state.
- **Confirmations.** Merge, rebase-update and convert-to-draft always ask first. Update-branch (merge) and ready-for-review run immediately.
- **The merge dialog lists only the merge methods the repo allows.**
- **Tests check external behaviour only.** That means what is on screen, which GraphQL requests went out, and what was written to the config and state files.

## Deliberately absent

- **File diffs and file lists**, anywhere, including diff hunks in review threads. This is a status dashboard, not a code-review tool.
- **Terminal hyperlinks (OSC 8).** Multiplexers drop them, and combined with app-handled clicks a link could open twice. Clicks are hit-tested by the app.
- **Writing to PR conversations:** comments, approvals and reviewer requests.
- **Re-running failed checks.** v2 candidate.
- **Desktop notifications.** v2 candidate.
- **More than one tag per PR**, or badges that don't change the lane. v2 candidate.
- **Manual card ordering inside a lane.** Cards sort by most recently updated.
- **Drag-and-drop.**
- **Custom search queries or tabs** beyond Board, Review requests and Archived.
- **Syncing tags across machines.**
- **Any auth of its own:** token prompts, OAuth flows, token fields in config.
- **Modifying workflow files** in the user's repos.
- **Creating repos, the tap repo, secrets or releases** from the build. Publishing is a manual step.
