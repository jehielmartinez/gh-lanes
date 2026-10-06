# Dev loop

Conventions are the claude-kit standard. This file carries what that cannot decide for this repo.

## Areas

| Area | Paths | Gate |
|---|---|---|
| `app` | `*.go`, `go.mod`, `go.sum`, `internal/**`, `testdata/**` | `gofmt -l .` prints nothing, `go vet ./...`, `go test ./...` |
| `infra` | `.github/workflows/**` | nothing automated |
| `infra` | `.goreleaser.yaml` | `goreleaser check` |
| `docs` | `docs/**`, `README.md`, `AGENTS.md`, `CLAUDE.md`, `CODING-STANDARDS.md` | nothing automated |

The `main` package sits at the repo root so the `gh` extension precompile action builds it as `gh-lanes`. Every other package lives under `internal/`. The tracer bullet (#2) sets the exact package layout. Later tickets follow it and do not reorganise it.

## Tracker

None.

## Stop rules

- **Only the GitHub layer talks to the network.** No other package imports `net/http`, `go-gh`'s API clients or any GraphQL client. If a feature seems to need a request from somewhere else, add a method to the GitHub layer's interface instead.
- **Network calls never run on the UI thread.** Every fetch and mutation is a `tea.Cmd` that returns a message. Never call the GitHub layer from `Update` or `View`.
- **Tests go through the one seam.** That means the real root model in `teatest`, with the fake GraphQL transport, a temp config dir and the controllable clock. Do not unit-test private helpers, export internals for tests, or mock packages inside the app. If a behaviour can't be reached through the seam, extend the seam, for example with a fake opener.
- **Do not add dependencies the spec doesn't name.** The spec names Bubble Tea, Bubbles, Lip Gloss, `go-gh`, `teatest` and a YAML library. Anything else, a markdown renderer included, needs a written reason in the PR. An off-the-shelf markdown renderer is allowed only if link screen positions stay trackable. Otherwise write a small custom one.
- **Fixtures are scrubbed.** Use placeholder owners, repos, logins, URLs and node IDs only (`octo-org/sample-repo`, `user-a`). Never record against a real account and commit the result as is. No tokens, no real org, company or person names.
- **`UNKNOWN` mergeability is "checking…".** Never fall back to treating it as a conflict, or as mergeable.
- **Do not invent fallbacks for auth.** Auth and host come from `go-gh` (which honours `GH_TOKEN` and `GH_HOST`). If there is no login, exit with the `gh auth login` message. Do not prompt for a token, read one from the config, or try another source.
- **Don't build what the spec puts out of scope**, even when it's one line away. That covers re-running checks, commenting, approving, notifications, extra tabs and custom searches. Note it in the PR as a v2 candidate instead.
- **Never create GitHub resources.** That means repos, the tap repo, secrets, releases and tags. #15 writes the config files only.

## Worktree carry

No `.worktreeinclude`. Nothing untracked is needed. The Go module and build caches are global (`GOMODCACHE`, `GOCACHE`), so a fresh checkout builds as is.

The worktree is ready when `go.mod` exists. The exception is #2, which creates it.

## Seams

The spec defines one seam, and every behaviour is tested through it:

- the root app model, driven by `teatest`, asserting on rendered screen output
- a fake GraphQL HTTP transport injected into the GitHub layer. It replays fixtures per query and records every request, so a test can assert the exact query or mutation and its variables. It also simulates errors, rate limits, `UNKNOWN` mergeability and paging.
- a temporary config directory, seeded and read back to assert on the config YAML and state JSON
- a controllable clock with a fixed timezone, for refresh ticks, relative ages and local-time formatting
- a fake URL opener, for link clicks and `O`, asserting on the URL it received

## Overrides

- **The rulebook lists live in `AGENTS.md`, not `CLAUDE.md`.** `## Non-negotiables` and `## Deliberately absent` are in `AGENTS.md` at the repo root. `CLAUDE.md` holds only `@AGENTS.md`, so the lists load from either name. The repo keeps its agent instructions in the tool-neutral file, so edit `AGENTS.md` and never add rules to `CLAUDE.md`.
