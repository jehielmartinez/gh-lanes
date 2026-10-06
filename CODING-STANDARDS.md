# Coding standards

How to write code here. What the product is lives in `docs/spec.md`, and what a change must never do lives in `AGENTS.md`. This file holds the principles a change is expected to follow, so review can point at a rule instead of at taste.

**One rule governs the rest: Easy To Change (ETC).** Every principle below follows from it. Where two of them conflict, the one that makes the next change easier wins. None is a purity test.

---

## 1. A comment carries only what the code cannot

The code says what it does. Naming, types and small functions are the main tools. A comment is what's left when those are used up.

**DRY applies to comments**, and the test is ETC:

> If changing the code forces you to change the comment, the comment is a second copy of the code. Delete it.
> If the comment stays true and useful after a refactor, it carries something the code can't. Keep it.

Nothing type-checks prose, so a duplicated comment doesn't fail. It quietly starts lying. Four kinds survive the test:

1. **Why this, and not the obvious thing.** The rejected alternative is invisible in the code by definition.
2. **The failure that bought the line.** Cite the issue.
3. **A deliberate absence.** Something that isn't there can't explain itself.
4. **A constraint from outside the file.** For example, a GitHub API quirk (`UNKNOWN` mergeability while GitHub computes it), a terminal or multiplexer limitation, or a policy the code obeys but can't state.

Delete on sight: step narration, comments that restate a name or type, section headers inside a function, inventories and counts, changelogs, and commented-out code.

Doc comments on exported identifiers follow Go convention: start with the name, say what it promises, and say nothing about how it does it.

A comment that passes the test earns its place, not its length: one or two sentences. If it needs a paragraph, the reasoning belongs in `docs/` with a pointer to it. Before writing a comment, try to make it unnecessary with a better name, a named constant, or an extracted function whose signature states the thing.

## 2. Pure core, effectful shell

Network, filesystem, clock and the OS opener live at the edges. The logic between them stays pure. Domain, board assembly, activity and link extraction take values and return values.

A function that takes `now time.Time` as a parameter can be reasoned about. One that calls `time.Now()` cannot. The clock is read in one place, and everything downstream gets the value.

## 3. Deciding, orchestrating and composing are three different jobs

- **Decide:** pure functions that take values and return values. For example, building lanes from PRs and tags, turning `mergeStateStatus` into a sentence, or deciding whether an action is available.
- **Orchestrate:** `Update`. It routes messages, starts `tea.Cmd`s and sequences effects. It asks the decide layer what to do and doesn't work it out inline.
- **Compose:** `View`. It reads state and maps it to strings. No decisions, no I/O.

Mixing these jobs is what makes a file long. Concerns that only sit next to each other in one scope are not separated: change any one of them and you must read them all.

Name a package for what it decides: `board`, `activity`, `links`. Never `util`, `helpers` or `common`, which name nothing.

**Tripwires:** a function past about 80 lines, an `Update` whose `switch` has outgrown one screen, or a `View` that branches on more than presentation. Each is a *question*: is this one job? Then apply ETC. A tightly sequenced state machine that reads better whole stays whole. Say why in a comment and move on. A tripwire is never an automatic failure.

## 4. Inject boundaries; never mock a package

When a boundary needs faking for a test, pass it in. The GitHub layer takes its HTTP transport. The app takes its clock, config directory and URL opener. Code that constructs its own client inside needs the whole world stood up to test.

Define an interface where it is consumed, keep it small, and give it only the methods that caller uses.

## 5. Values by default, mutation on purpose

Return new values rather than mutating arguments, and never mutate a slice or map you were given or returned. Bubble Tea models are values for a reason: `Update` returns the next model.

A plain `for` loop is idiomatic Go and usually the clearest form. Use whichever reads best, and ETC decides.

## 6. Dependencies point one way

`main` → `ui` → (`board`, `activity`, `links`, `store`, `github`) → `domain`. `domain` imports nothing from this module. Nothing imports `ui`. Only `github` imports the network. If reaching for a type drags in an HTTP client, the layering is wrong. `internal/` keeps all of it private to the binary.

## 7. One source of truth for anything referenced twice

That covers keybindings (the help footer and help screen are generated from the keymap), GraphQL field selections, theme colours, default tags, the schema `version`, and search strings. Derive the second use from the first instead of typing it again. A hand-typed duplicate compiles clean and is silently orphaned on a rename.

## 8. Orthogonal: unrelated things stay independently changeable

This is DRY's other half. DRY forbids one piece of knowledge living in two places. Orthogonality forbids two unrelated pieces being so entangled that neither can change alone. Both come down to ETC: a given change should land in one place, and only there.

The test is blast radius. Name a plausible change and count the packages you must *understand*, not just touch, to make it safely. Package-level mutable state, hidden ordering between messages, and a parameter whose meaning depends on the caller each couple things that have no reason to move together, and every later change pays interest on them.

§3 (one job per function), §4 (injected boundaries) and §6 (one-way dependencies) are the working tools. Orthogonality is the property they exist to buy.

## 9. Degrade, don't explode

A malformed response, a missing field or an unreadable state file becomes a rendered state, never a panic. A failed refresh keeps the last good board and says it is stale. An empty board means there are no PRs. It never stands in for an error.

Return errors, wrap them with `%w` and context, and surface them in the status bar. `panic` is for programmer errors only, and never for anything GitHub or the filesystem can cause.

## 10. Test behaviour, through the seam

This repo has one test seam (see `docs/agents/dev-loop.md`): the real root model under `teatest`, with the fake transport, temp config dir, fixed clock and fake opener. Every branch that matters, especially the failure branch, is reached through it. Assert on what the user sees, the requests that went out, and the files that were written.

Test any behaviour a bug report named. A fix without a test that fails before the fix is incomplete.

Don't test framework plumbing or styling, or anything whose test would restate the implementation. If something is hard to reach through the seam, that's a design signal, not a reason to test internals.

## 11. Delete rather than flag

A superseded abstraction is removed, not left behind a flag or a comment. Don't add parameters, interfaces or layers for needs that don't exist yet. Git remembers.

## 12. When a rule is wrong, change it everywhere

If a rule costs more than it prevents, change it. But change it repo-wide and say so in the commit. A convention that holds in half the codebase is worse than either alternative.

---

## House rules

Local to this repo, and not derivable from the principles above.

- **Formatting:** `gofmt` output, always. No hand-aligned exceptions.
- **Untrusted text is sanitised in one place.** PR text passes through a single sanitiser that strips escape and control sequences on its way from the GitHub layer to the UI. Views never sanitise ad hoc, and never render raw API strings.
- **Styling goes through the theme.** Colours are Lip Gloss adaptive colours defined in the theme. A literal colour in a view needs a comment saying why the theme can't express it.
- **GraphQL lives in the GitHub layer.** Queries are constants next to the code that maps their responses. Mapping produces `domain` values, and no GraphQL response type leaks past `github`.
- **Fixtures are recorded, then scrubbed.** Every owner, repo, login, URL and node ID is a placeholder before commit (see `AGENTS.md`).
- **Commits and PRs** follow the claude-kit standard: `<type>(<area>): <lowercase summary>`.

The prohibitions that outrank all of this (untrusted input, no credentials, no diffs, local-only tags) are in `AGENTS.md`.
