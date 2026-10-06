# lanes — specification

Status: **specified, not built.**

## Problem Statement

I author pull requests across several organizations and repositories. To find out where each one
stands, I have to open many browser tabs and check each PR's checks, reviews, mergeability and
conflicts. GitHub has no single place that shows all of my open PRs with their live status, and I
can't group them by where they are in *my* workflow (in progress, in review, being tested, ready to
demo). Doing routine things like updating a branch, merging, or switching between draft and ready
means leaving the terminal and clicking through the web UI for each PR.

## Solution

`lanes` is a terminal control center for the open pull requests you authored. It runs as `gh lanes`
(a `gh` extension) or as a standalone `lanes` binary installed from Homebrew, and uses the account
`gh` is already logged in to.

- It finds every open PR you authored on every org and repo, without you naming any of them.
- It shows them as a kanban board: each lane is a **tag** you define locally (In Progress, Review,
  Testing, Demo, Done, or your own). Tags never touch GitHub.
- The board stays live: checks, review decision, mergeability, conflicts and draft state refresh
  automatically, and cards with new activity get a marker.
- Opening a PR shows a detail modal with merge status, checks, the description, and a single
  conversation timeline of comments, reviews and review threads, all with local timestamps. It never
  shows file diffs.
- You can act on a PR without leaving the terminal: update the branch (merge or rebase), merge with
  any method the repo allows, enable or disable auto-merge, mark ready or convert to draft, open the
  PR in the browser, and open any link in it.
- A second tab lists PRs where your review is requested, with the same cards and modal.

## User Stories

### Setup and launch

1. As a developer, I want to install the tool with `gh extension install`, so that it fits into my
   existing `gh` workflow.
2. As a developer, I want to install it with `brew install`, so that I can run it as a plain `lanes`
   command.
3. As a developer, I want the tool to reuse my existing `gh` login, so that I don't manage another
   token.
4. As a developer, I want the tool to honor `GH_TOKEN` and `GH_HOST`, so that it behaves like `gh`
   in scripted or non-default setups.
5. As a developer who isn't logged in to `gh`, I want a clear message telling me to run
   `gh auth login`, so that I know how to fix it.
6. As a developer, I want the tool to start almost instantly, so that I can open it as casually as
   `git status`.
7. As a developer, I want `--version` and `--config` flags, so that I can check what I'm running and
   point it at a different config.

### Finding PRs

8. As a developer, I want every open PR I authored, across every org and personal repo, to appear
   without configuration, so that I never miss one.
9. As a developer, I want PRs in archived repos left out, so that dead work doesn't clutter the
   board.
10. As a developer, I want a new PR I just opened to appear in the Untagged lane on the next refresh,
    so that Untagged works as my inbox.

### Tags and lanes

11. As a developer, I want a set of default tags (In Progress, Review, Testing, Demo, Done) on first
    run, so that the board is useful immediately.
12. As a developer, I want to create my own tags, so that the lanes match how I actually work.
13. As a developer, I want to rename a tag without losing which PRs are in it, so that I can refine
    names freely.
14. As a developer, I want to recolor a tag, so that lanes are easy to tell apart.
15. As a developer, I want to reorder tags, so that lanes appear left to right in my workflow order.
16. As a developer, I want deleting a tag to move its PRs to Untagged, so that no PR disappears.
17. As a developer, I want to mark a tag as terminal (like Done), so that finished work is displayed
    differently.
18. As a developer, I want an Untagged lane that's always first and can't be deleted, so that new
    PRs always have somewhere to land.
19. As a developer, I want tags kept on my machine only, so that teammates never see my private
    workflow labels.
20. As a developer, I want the tag config in a readable YAML file, so that I can edit or back it up
    by hand.
21. As a developer, I want each PR in exactly one lane, so that its lane is its status with no
    ambiguity.

### Board

22. As a developer, I want lanes shown as columns with a name, color and count in the header, so
    that I can see the shape of my work at a glance.
23. As a developer, I want each card to show the repo and number, the title, check status, review
    decision, mergeability and age, so that I can triage without opening it.
24. As a developer, I want cards sorted by most recently updated within a lane, so that active work
    is on top.
25. As a developer, I want the board to scroll horizontally when lanes don't fit, keeping the
    focused lane visible, so that the tool works in narrow terminals.
26. As a developer, I want each lane to scroll vertically on its own, so that a long lane doesn't
    push the others around.
27. As a developer, I want to move a card one lane left or right with a single key, so that
    retagging is fast.
28. As a developer, I want a "move to…" picker, so that I can jump a card to any lane directly.
29. As a developer, I want to navigate lanes and cards with both vim keys and arrow keys, so that it
    feels natural either way.
30. As a developer, I want to click a card to select it and double-click to open it, so that I can
    use the mouse when it's easier.
31. As a developer, I want a help footer and a full help screen, so that I can discover keys.

### Merged and closed PRs

32. As a developer, I want a tagged PR to stay on the board with a Merged or Closed badge after it
    merges, so that post-merge work (testing, demo) stays visible.
33. As a developer, I want untagged PRs to drop off once they're no longer open, so that the inbox
    only holds live work.
34. As a developer, I want to archive a card with one key, so that I decide when it leaves the
    board.
35. As a developer, I want an archived PR to stay gone, so that it doesn't come back on the next
    refresh.
36. As a developer, I want an archived PR that is reopened to come back to Untagged, so that I
    notice it's live again.
37. As a developer, I want merged and closed PRs in a terminal lane shown dimmed, so that finished
    work recedes visually.

### Keeping data fresh

38. As a developer, I want the board to refresh automatically every 60 seconds, so that statuses
    stay current without me doing anything.
39. As a developer, I want to change the refresh interval in the config, so that I can trade
    freshness for API usage.
40. As a developer, I want a key to refresh immediately, so that I can check right after pushing.
41. As a developer, I want the UI to stay responsive during a refresh, so that I can keep working.
42. As a developer, I want a spinner and an "updated Ns ago" indicator, so that I know how fresh the
    data is.
43. As a developer, I want errors (offline, expired login) shown in the status bar while the last
    good data stays on screen, marked stale, so that a network blip doesn't wipe my board.
44. As a developer, I want a warning when my API rate-limit budget is low, so that I understand why
    refreshes might fail.
45. As a developer, I want "checking…" shown while GitHub is still computing mergeability, so that I
    don't mistake an unknown state for a conflict.

### Change awareness

46. As a developer, I want an activity dot on cards whose comments, reviews, checks, mergeability,
    review decision, draft state or open/merged/closed state changed since I last opened them, so
    that I know where to look.
47. As a developer, I want the status bar to show how many PRs changed, so that I know at a glance
    whether anything needs me.
48. As a developer, I want opening a PR to clear its dot, so that the marker means "unseen".
49. As a developer, I want a "new" marker for PRs that appear after the first run, without every
    card lighting up on first launch, so that the first run is calm.

### Detail modal

50. As a developer, I want to open a PR in a large overlay that scrolls and closes with Esc, so that
    I can dig in without losing my place on the board.
51. As a developer, I want the header to show the repo and number, title, author, `head → base`
    branches, state badge, and created and updated times, so that I have the context up top.
52. As a developer, I want mergeability and merge state explained in plain sentences (for example
    "Behind `main`. Update branch available"), so that I don't have to decode GitHub's enums.
53. As a developer, I want the review decision and auto-merge status shown, so that I know what's
    blocking a merge.
54. As a developer, I want checks grouped by workflow with failures first, status icons and
    durations, so that I can see what broke right away.
55. As a developer, I want a summary line like "12 passed · 1 failed · 2 running", so that I get the
    overall picture in one line.
56. As a developer, I want each check's details link to be clickable, so that I can jump to the
    failing log.
57. As a developer, I want the PR description rendered as readable markdown, so that it's pleasant
    to read in a terminal.
58. As a developer, I want comments, reviews and review threads in one timeline sorted by time, so
    that I can follow the conversation in order.
59. As a developer, I want review threads to show their `file:line` and resolved or unresolved
    state, with resolved threads collapsed, so that I can focus on what's open.
60. As a developer, I want every timestamp in my local timezone, as both an absolute date and a
    relative age, so that I don't do timezone math.
61. As a developer, I want the modal's data to refresh while it's open, so that I see new comments
    and check results live.
62. As a developer, I never want to see file diffs or file lists, so that the tool stays a status
    dashboard and not a code-review tool.

### Links

63. As a developer, I want to click any link in the description, comments, reviews or threads to
    open it in my browser, so that following references is one click.
64. As a developer, I want links underlined, so that I can tell what's clickable.
65. As a developer, I want markdown links to show their text, and bare URLs to be detected, so that
    both kinds are usable.
66. As a developer, I want links that wrap onto the next line to still be clickable, so that long
    URLs work.
67. As a developer, I want a link picker listing every URL in the PR with its source and author, so
    that I can open links from the keyboard.
68. As a developer, I want link clicks handled by the app itself, so that they work inside terminal
    multiplexers that don't pass terminal hyperlinks through.
69. As a developer, I want to still select text with Shift+drag, so that I can copy things while
    mouse support is on.
70. As a developer, I want a key to open the PR itself in the browser, so that I can go to GitHub
    when I need the full UI.

### Actions

71. As a developer, I want to update a PR's branch from its own base branch (whatever it is), so
    that I don't have to check it out locally.
72. As a developer, I want a separate rebase update that asks for confirmation, so that I don't
    rewrite a branch by accident.
73. As a developer, I want a merge dialog that offers only the merge methods the repo allows, so
    that I can't pick an invalid one.
74. As a developer, I want a delete-branch toggle in the merge dialog that defaults to the repo's
    setting, so that cleanup matches the repo's conventions.
75. As a developer, I want to enable auto-merge when checks are pending or reviews are missing and
    the repo allows it, so that the PR merges itself once ready.
76. As a developer, I want to disable auto-merge, so that I can stop a queued merge.
77. As a developer, I want to mark a draft ready for review, so that I can hand it off from the
    terminal.
78. As a developer, I want to convert a PR back to draft, with a confirmation, so that I know it may
    dismiss review requests.
79. As a developer, I want actions offered only when GitHub says they're possible, so that I don't
    hit avoidable errors.
80. As a developer, I want the PR to refresh right after an action and a toast showing success or
    GitHub's error message, so that I know the result.
81. As a developer, I want merge and rebase-update to always confirm, and the other actions to run
    immediately, so that only destructive actions slow me down.
82. As a developer, I want actions available from both the board and the modal, so that I don't
    have to open a PR to act on it.

### Review requests

83. As a reviewer, I want a separate tab listing open PRs where my review is requested, sorted by
    last update, so that I see my review queue next to my own work.
84. As a reviewer, I want those PRs to use the same cards and modal, so that there's nothing new to
    learn.
85. As a reviewer, I want actions I can't perform on someone else's PR (merge, draft/ready) hidden
    or disabled, so that the UI only offers what will work.
86. As a reviewer, I want review-request PRs kept out of my tagged lanes, so that my workflow lanes
    only hold my own work.

### Local data

87. As a developer, I want state saved atomically, so that a crash never corrupts my tags.
88. As a developer, I want PRs tracked by a stable ID, so that renames, transfers and title changes
    don't lose their tag.
89. As a developer, I want the config and state files versioned, so that future releases can
    migrate them safely.
90. As a developer, I want `$XDG_CONFIG_HOME` honored, with `~/.config` used on macOS too, so that
    my config lives where my other CLI configs do.

## Implementation Decisions

### Stack and distribution

- **Go + Bubble Tea**, with Bubbles (lists, viewport, text input, spinner, help), Lip Gloss
  (styling, adaptive light and dark colors) and `go-gh` (auth, host resolution and GraphQL client
  reused from `gh`'s config). Chosen mainly for distribution: one static binary, fast startup, and
  standard tooling for releases.
  - Rejected: Python + Textual (Homebrew formula has to pin every dependency, slower startup) and
    TypeScript + Ink (no built-in modal, scrolling or focus, large compiled binary).
- **Distribution:** the repo is named `gh-lanes` so it works as a `gh` extension. GoReleaser builds
  darwin and linux binaries for arm64 and amd64, and publishes a Homebrew formula named `lanes` to a
  tap repo. The official `gh` extension precompile action produces extension assets from the same
  tag. The formula's caveats say `gh auth login` is required.
- Creating repos, secrets or releases is a manual, public step and is not part of the build. Release
  credentials live only in repository secrets, never in files.

### Modules

- **GitHub layer:** the only module that talks to the network. It exposes a small interface:
  list PRs for a search string, fetch PRs by node ID, fetch one PR's full detail, and one method per
  mutation. It maps GraphQL responses into domain values. Its HTTP transport is injectable; this is
  the test seam.
- **Domain:** PR, check summary, review, thread, comment and merge status types. No I/O and no UI.
  Includes turning `mergeable` and `mergeStateStatus` into plain sentences, and gating which actions
  are available.
- **Store:** loads and saves the config file (YAML, user-editable) and the state file (JSON,
  machine-written) in the config directory. Writes are atomic (temp file, then rename). Both files
  carry a schema `version`.
- **Board assembly:** combines fetched PRs with tags, assignments, the archived list and snapshots
  to produce lanes. Implements the merged/closed retention rules.
- **Activity:** compares a PR's current data with its last-seen snapshot and decides whether it
  changed, or is new.
- **Links:** extracts URLs (markdown links and bare URLs) from text, and records where they're drawn
  for click hit-testing.
- **UI:** a root model (tabs, refresh ticker, toasts, message routing) plus board, review list,
  detail modal, reusable picker (move to, links, merge method), tag manager, confirmation dialog and
  theme.

### Data flow

1. A tick, a refresh key or a finished action sends a fetch command that runs off the UI thread.
2. The GitHub layer returns domain values as a message.
3. The root model merges them with the store to build lanes.
4. Views render from that state.

Mutations are commands returning a result message, which triggers a single-PR refresh.

### Searches

- Board: `is:pr is:open author:@me archived:false`.
- Review requests: `is:pr is:open review-requested:@me archived:false`.
- Tracked PRs that are no longer open are fetched each refresh by node ID, since the open search no
  longer returns them.

### List query fields per PR

- Identity and state: id, number, title, url, draft, state, merged, created/updated/merged/closed
  times, author login.
- Branches: base and head ref names.
- Merge and review: `mergeable`, `mergeStateStatus`, `reviewDecision`, auto-merge request,
  `viewerCanUpdate`.
- Repository: name with owner, allowed merge methods, auto-merge allowed, delete-branch-on-merge.
- Checks: the last commit's status check rollup, with check runs (name, status, conclusion, start
  and end times, details URL, workflow name) and status contexts.
- Activity counts: comment and review totals and latest reviews, for snapshots.

The detail query (on modal open) adds the body, comments, reviews, and review threads with their
comments. Any connection reporting more pages is paged through. `UNKNOWN` mergeability is shown as
"checking…" and never treated as a conflict.

### Mutations

| Action | Mutation | Rules |
|---|---|---|
| Update branch (merge) | `updatePullRequestBranch` with MERGE | Uses the PR's own base branch. |
| Update branch (rebase) | `updatePullRequestBranch` with REBASE | Separate key, always confirms. |
| Merge | `mergePullRequest` | Dialog lists only allowed methods. Delete-branch toggle defaults to the repo setting; branch deleted with `deleteRef` after success unless the repo does it itself. Always confirms. |
| Enable/disable auto-merge | `enablePullRequestAutoMerge` / `disablePullRequestAutoMerge` | Offered in the merge dialog when checks are pending or reviews are missing and the repo allows it. |
| Ready for review | `markPullRequestReadyForReview` | Drafts only. |
| Convert to draft | `convertPullRequestToDraft` | Non-drafts only, confirms. |
| Open in browser / open link | none | OS opener (`open` on macOS, `xdg-open` on Linux). |

Actions are gated on `viewerCanUpdate`, repo merge settings and PR state.

### Local files

- **Config file** (user-editable YAML): `version`, `refresh_interval` (default 60s), and an ordered
  list of tags, each with a stable `id`, `name`, `color` and optional `terminal` flag. Assignments
  reference the tag ID, so renames are safe. The app reloads it on start and after saving from the
  tag manager.
- **State file** (machine-written JSON): `version`, assignments (PR node ID → tag ID), the archived
  PR node IDs, and per-PR snapshots (seen-at time, overall check state, mergeability, review
  decision, comment and review counts, draft flag, state).
- Neither file ever contains a token or credential. Auth stays in `gh`'s own storage.

### Links and mouse

- Mouse reporting uses Bubble Tea's cell-motion mode.
- The modal records the screen region of every rendered URL, including wrapped ones, and opens the
  URL on a left click. Links are underlined.
- Terminal-native hyperlinks (OSC 8) are not used: they may be dropped by multiplexers, and using
  them alongside app-handled clicks could open a link twice.
- The markdown renderer must keep link positions trackable. If an off-the-shelf renderer can't,
  use a lightweight custom one.
- Before opening, a URL is checked to have an `http` or `https` scheme, and is passed to the OS
  opener as one argument, never through a shell. PR content is untrusted input.

### Keybindings

| Key | Where | Action |
|---|---|---|
| `tab` / `shift+tab` | global | switch Board → Review requests → Archived |
| `h` `l` / `←` `→` | board | focus previous/next lane |
| `j` `k` / `↓` `↑` | board, lists | select card |
| `H` `L` / `<` `>` | board | move card one lane left/right |
| `m` | board | "move to…" picker |
| `enter` | board, lists | open detail modal |
| `x` | board | archive card |
| `t` | global | tag manager |
| `r` | global | refresh now |
| `o` | board, modal | link picker |
| `O` | board, modal | open PR in browser |
| `u` / `U` | board, modal | update branch (merge / rebase) |
| `M` | board, modal | merge dialog |
| `d` | board, modal | toggle draft ↔ ready |
| `esc` | modal, pickers | close |
| `?` | global | full help |
| `q` / `ctrl+c` | global | quit |

## Testing Decisions

- **Good tests check external behavior only:** what the user sees on screen, which GraphQL requests
  went out, and what ended up in the config and state files. They don't reach into model internals
  or private helpers, so a refactor that keeps behavior intact keeps tests green.
- **One seam.** Tests drive the real root app model through `teatest` with:
  - a **fake GraphQL HTTP transport** injected into the GitHub layer. It replays recorded response
    fixtures per query and records every request, so tests can assert which mutation was sent with
    which arguments, and can simulate errors, rate limits, `UNKNOWN` mergeability and paging;
  - a **temporary config directory**, so tests can seed config and state files and assert on what
    was written;
  - a **controllable clock**, so refresh ticks, relative ages and local-time formatting are
    deterministic (with a fixed timezone).
- Everything is covered through that seam: response mapping, lane assembly, retention after
  merge/close, archive behavior, activity markers, tag manager CRUD, action gating, confirmations,
  merge dialog options, link picker, and error and stale-data handling.
- **Fixtures** are recorded GraphQL responses scrubbed of real data: placeholder owners, repos,
  logins, URLs and IDs only. No real org, company or person names, and no tokens, in fixtures or
  test output.
- Mouse link clicks are tested by sending mouse events at the recorded coordinates of a rendered
  link and asserting the fake opener received the URL.
- **Prior art:** none in this repo yet; this spec establishes the pattern. CI runs `go test ./...`
  and `go vet` on every push.

## Out of Scope

- File diffs or file lists of any kind.
- Desktop notifications (v2 candidate).
- Flags or extra tags beyond the single lane tag (v2 candidate: badges like Blocked that don't
  change the lane).
- Manual card ordering inside a lane.
- Drag-and-drop.
- Custom search queries or extra tabs beyond Board, Review requests and Archived.
- Writing comments, approving PRs or requesting reviewers.
- Syncing tags across machines.
- Re-running failed checks (cheap v2 candidate).
- Creating the GitHub repo, tap repo, secrets or releases.

## Further Notes

- **Performance:** the dataset is tens of PRs, so UI cost is negligible. Latency is the GitHub API
  (about 0.5–2 s per refresh), which always runs off the UI thread. A refresh costs a few points of
  the 5,000-point hourly GraphQL budget.
- **Permissions:** reads and actions need the `repo` scope that `gh auth login` grants by default.
  The tool never modifies workflow files.
- **Security:** the tool stores no credentials, opens only `http`/`https` URLs, never runs PR
  content through a shell, and treats all PR text as untrusted when rendering (terminal escape
  sequences in bodies and comments are stripped before display).
- **Build order:** project skeleton and CI → GitHub layer with fixtures → store → board → modal and
  links → actions → review requests tab → tag manager → packaging and README.
- **License:** to be chosen at publish time (MIT suggested).
