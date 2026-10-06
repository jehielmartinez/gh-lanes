# lanes — specification

`lanes` is a terminal "control center" for the open pull requests you authored on GitHub. It shows
every PR across every org and repo, lets you sort them into kanban lanes using your own local tags,
keeps their status live, and lets you act on them (update branch, merge, draft/ready, open links)
without leaving the terminal. It never shows file diffs.

Status: **specified, not built.** Every decision below was settled in a design interview on
2026-10-06. Where a decision has a reason, the reason is recorded so a later change can weigh what it
gives up.

---

## 1. Original request

Captured as stated, so the spec can be checked against it:

- A terminal tool, a control center TUI, to visualize important info about open PRs.
- Requires a `gh`-authenticated user on the machine.
- Track the status of all of my PRs across different repositories, ideally without having to name
  the org.
- Assign tags to PRs that exist only inside the tool: In Progress, Demo, Testing, Review, etc., and
  the user can define their own.
- Group PRs by tag in lanes, like GitHub Projects or Jira boards.
- Keep the information on every PR up to date.
- No file diffs, but a nicely formatted view of: Actions/check status, PR status, merge status,
  conflicts.
- Selecting a PR opens a modal with the above plus comments and reviews, with localized timestamps.
- Actions from the tool: update branch, merge the PR, open links from comments or the PR body, mark
  ready for review, convert to draft, etc.

---

## 2. Environment facts (verified, not decided)

Checked on the author's machine on 2026-10-06:

- `gh` is logged in to `github.com` with scopes `gist`, `read:org`, `repo`, `workflow`. That's enough
  for every read and every action in this spec (merge and update-branch need `repo`; workflow-file
  changes are never made).
- Finding PRs across orgs without naming them works: `gh search prs --author=@me --state=open`
  returned 13 open PRs across 4 owners (two orgs and a personal account). The GraphQL equivalent is
  `search(query: "is:pr is:open author:@me", type: ISSUE)`.
- The terminal is **Ghostty running inside the herdr multiplexer**. Whether herdr passes OSC 8
  hyperlink sequences through is unknown, which drove decision D10.
- Go and Rust weren't installed (Node 24, Bun and Python 3 were). Building requires
  `brew install go` first.
- Name check (Homebrew formulae and GitHub repos): `lanes` has no Homebrew formula and no `gh-lanes`
  extension.

---

## 3. Decisions

### D1. Stack: Go + Bubble Tea

- **Libraries:** Bubble Tea (runtime), Bubbles (lists, viewport, text input, spinner, help), Lip Gloss
  (styling and layout), `github.com/cli/go-gh/v2` (auth, host and GraphQL client reused from `gh`'s
  own config).
- **Why:** the deciding factor was publishing. GoReleaser turns a git tag into multi-platform binaries
  and a Homebrew formula automatically, and GitHub has an official Action for precompiled Go `gh`
  extensions.
- **Rejected: Python + Textual.** It has the most built-in UI (modals, scroll containers, CSS
  layout), but a Homebrew formula for it has to declare and pin every Python dependency, regenerated
  on every bump. It also starts about 0.5 s slower (Go ~10–20 ms, Textual ~300–600 ms) and uses more
  memory (~15–30 MB vs ~60–100 MB).
- **Rejected: TypeScript + Ink.** It's the most familiar, but it has no built-in modal, scrolling or
  focus management, and Homebrew distribution means either a Node dependency or a 60–90 MB
  `bun build --compile` binary.
- **Performance note:** at this scale (tens of PRs) the UI framework doesn't matter. The real latency
  is the GitHub API (about 0.5–2 s per refresh), and it always runs off the UI thread.
- **Cost accepted:** the most code of the three options, in the language the author knows least. The
  modal overlay is the main piece that has to be built by hand.

### D2. Distribution: Homebrew tap + `gh` extension

- The repo is named **`gh-lanes`**, which a `gh` extension requires. Users run it as `gh lanes`, or
  as `lanes` when installed from Homebrew.
- **Homebrew:** GoReleaser publishes a formula named `lanes` to a tap repo
  (`jehielmartinez/homebrew-tap`), so users install with `brew install jehielmartinez/tap/lanes`.
- **gh extension:** `cli/gh-extension-precompile` builds release assets, so users install with
  `gh extension install jehielmartinez/gh-lanes`.
- Both channels build from the same tag.
- The binary must work standalone (from Homebrew) as well as an extension. `go-gh` handles both: it
  reads `gh`'s stored token and host either way, and honors `GH_TOKEN` and `GH_HOST`.
- **Not done until asked:** creating the GitHub repo, the tap repo, secrets, or any release. Those
  steps are public. The GoReleaser config and release workflow files are written as part of the
  build.

### D3. Scope: your PRs on the board, review requests in a separate tab

- **Board tab:** PRs where you're the author.
  - Search: `is:pr is:open author:@me archived:false`.
  - Also fetched: tracked PRs that are no longer open (see D6).
- **Review requests tab:** `is:pr is:open review-requested:@me archived:false`.
  - It has no tags and no lanes: a single list sorted by last update.
  - It uses the same card rendering and the same detail modal. Actions that need to be the PR's
    author (draft/ready, merge) are hidden or disabled when you can't perform them, based on
    `viewerCanUpdate` and the repo's permissions.
- **Why:** tags describe *your* workflow, so a teammate's PR has no place in your "Demo" lane. The
  review queue is the same fetch with a different search, so it's nearly free.
- **Rejected:** mixing review requests into the board (they don't fit the tags), and arbitrary
  user-configured search queries (not needed yet, but easy to add later because the search string is
  the only thing that differs).

### D4. Tags are local only

- Tags and their assignments never touch GitHub. No labels are created and teammates see nothing.
- Stored under `~/.config/lanes/` (see §6). `$XDG_CONFIG_HOME` is honored if set, and `~/.config` is
  used even on macOS rather than `~/Library/Application Support`.
- **In-app tag manager:** create, rename, recolor, reorder and delete tags.
  - Deleting a tag moves its PRs to Untagged.
  - Renaming keeps assignments, because tags are referenced by a stable ID, not by name.
- The config file is plain YAML and may also be edited by hand. The app reloads it on start and after
  saving from the tag manager.
- **Default tags on first run:** In Progress, Review, Testing, Demo, Done. All are editable.

### D5. One tag per PR, and each tag is a lane

- Every PR is in exactly one lane. Moving a card means changing its tag.
- An **Untagged** lane is always first on the left and acts as the inbox for new PRs. It's built in,
  so it can't be deleted or renamed.
- Lane order follows the tag order set in the tag manager.
- Each tag has a `terminal` boolean, `true` for lanes like Done (see D6).
- **Rejected:** several tags per PR. Lanes need a single status, and a card showing up in two lanes
  breaks moving cards around.
- **Future (not v1):** "flags" (Blocked, Hotfix, and so on) as small badges on cards that don't affect
  the lane.

### D6. What happens after merge or close

- **Tagged PRs stay on the board** after merging or closing, with a `Merged` or `Closed` badge, until
  you archive them (`x`) or move them to a terminal lane.
- **Untagged PRs drop off** as soon as they leave the open-PR search.
- **Why:** labels like Demo and Testing often describe work that continues after the merge (QA on
  trunk, a demo from staging).
- **Consequence:** the open-PR search no longer returns tracked closed PRs, so each refresh also
  fetches them by node ID (`nodes(ids: [...])`).
- **Archive** removes the PR's assignment and adds its ID to an archived list, so it doesn't come
  back. An archived PR that is reopened on GitHub comes back to Untagged.
- A **terminal lane** (for example Done) shows its merged and closed PRs dimmed. Archiving from any
  lane takes one keypress.
- **Rejected:** dropping every PR on merge (Testing would vanish mid-QA), and expiring PRs after N
  days (hides things on a timer you didn't pick).

### D7. Refresh

- One batched GraphQL request for each list (board search, review search, tracked IDs) covers
  everything a card needs: checks, reviews, mergeability.
- **Auto-refresh every 60 s by default**, configurable as `refresh_interval` in the config.
  - Refresh runs off the UI thread as a Bubble Tea command, so the UI never blocks.
  - A spinner and a "updated 12s ago" indicator sit in the status bar.
- **`r`** refreshes immediately.
- **Opening the modal fetches that PR's full detail** (comments, reviews, threads) fresh. While the
  modal is open, the detail refreshes at the same interval.
- **Rate limit:** GitHub's GraphQL limit is 5,000 points per hour, and a refresh of this size costs a
  few points. The status bar shows the remaining budget when it drops below 10%.
- **Failure handling:** errors (offline, expired token) show in the status bar and keep the last good
  data on screen, marked stale. They never clear the board.

### D8. Change awareness: in-app new-activity markers

- Each PR has a **last-seen snapshot**, recorded when you last opened its modal.
- A card gets an **activity dot** when the current data differs from that snapshot in a way that
  matters:
  - new comments or reviews,
  - a change in the overall check status,
  - a change in mergeability or conflict state,
  - a change in review decision,
  - a change in draft or open/merged/closed state.
- The status bar shows the total ("3 PRs changed").
- Opening the modal clears the dot and updates the snapshot.
- **Before first view:** a PR you've never opened has no snapshot. It shows a "new" marker the first
  time it appears in a refresh after the initial run, so first launch doesn't light up every card.
- **Rejected for v1:** macOS desktop notifications. They only fire while the tool is running, need
  per-OS code, and complicate the Homebrew release. Candidate for v2.

### D9. Actions

Every action is a GraphQL mutation through `go-gh`.

- **Gating:** each action is offered only when GitHub says it's possible, using `viewerCanUpdate`, the
  repo's allowed merge methods, and the PR's state.
- **Feedback:** after an action, the PR refreshes right away. Success or the GitHub error message
  shows as a toast in the status bar.

| Action | Mutation | Notes |
|---|---|---|
| **Update branch (merge)** | `updatePullRequestBranch(updateMethod: MERGE)` | Brings in the PR's **own base branch**, whatever it is (not hard-coded to trunk or main). Offered when `mergeStateStatus == BEHIND`, or always with a "may be up to date" note. |
| **Update branch (rebase)** | `updatePullRequestBranch(updateMethod: REBASE)` | Separate key. Confirmation required, because a rebase rewrites the branch. |
| **Merge** | `mergePullRequest(mergeMethod: …)` | A confirmation dialog lists **only the methods the repo allows** (`mergeCommitAllowed`, `squashMergeAllowed`, `rebaseMergeAllowed`) and has a **delete branch** toggle, defaulting to the repo's `deleteBranchOnMerge`. Branch deletion uses `deleteRef` after a successful merge, unless the repo already deletes branches itself. |
| **Enable auto-merge** | `enablePullRequestAutoMerge` | Offered inside the merge dialog instead of a direct merge when checks are pending or required reviews are missing, and the repo has `autoMergeAllowed`. Disabling it is also available (`disablePullRequestAutoMerge`). |
| **Mark ready for review** | `markPullRequestReadyForReview` | Only for drafts. |
| **Convert to draft** | `convertPullRequestToDraft` | Only for non-drafts. Confirmation, because it may dismiss review requests depending on repo settings. |
| **Open PR in browser** | — | Opens `url` with the OS opener (`open` on macOS, `xdg-open` on Linux). |
| **Open a link** | — | See D10. |

Merge and rebase-update always ask for confirmation, and Enter confirms. The other actions run
immediately.

### D10. Links: the app handles clicks itself

- Mouse reporting is turned on with Bubble Tea's cell-motion mouse mode.
- When the modal renders the body, comments, reviews and threads, it **records where each URL is
  drawn** (row and column ranges, including links that wrap onto the next line). A left click on one
  of those regions opens the URL with the OS opener.
- **Underline links** so you can tell what's clickable. Markdown links (`[text](url)`) show their
  text and are clickable. Bare URLs are detected and clickable too.
- **Keyboard fallback:** `o` in the modal opens a **link picker** listing every URL in the PR (body,
  comments, reviews, threads), labeled with its source and author. Enter opens the selected one.
- **Selecting text:** Shift+drag (or the terminal's modifier key) selects text natively while mouse
  reporting is on. The author accepted this.
- The mouse also selects cards, opens the modal (double-click or click on a selected card), and
  scrolls lanes and the modal.
- **Why:** this is the only approach guaranteed to work through herdr. Terminal-native links (OSC 8)
  depend on every layer passing the sequence through, and they can break when Bubble Tea redraws or
  wraps text.
- **Rejected:** OSC 8 alone (may silently do nothing), and OSC 8 combined with app-handled clicks (one
  click could open the link twice).

### D11. Detail modal content

The modal is an overlay centered over the board that takes about 90% of the screen, scrolls as one
viewport, and closes with `Esc`. Sections from top to bottom:

1. **Header:**
   - `owner/repo#123`, title, author, and branches as `head → base`.
   - State badge: Open, Draft, Merged or Closed.
   - Created and updated times, in local time.
2. **Merge status:**
   - **Mergeability:** `mergeable` (MERGEABLE / CONFLICTING / UNKNOWN) and `mergeStateStatus`
     (CLEAN, BEHIND, BLOCKED, DIRTY, UNSTABLE, DRAFT, HAS_HOOKS, UNKNOWN), each turned into a plain
     sentence, for example "Conflicts with `trunk`. Resolve locally" or "Behind `trunk`. Update branch
     available".
   - **Reviews and auto-merge:** review decision (Approved, Changes requested, Review required) and
     auto-merge status.
3. **Checks:**
   - **Grouping:** check runs grouped by workflow name, with commit status contexts in their own
     group.
   - **Each line:** a status icon (✓ success, ✗ failure, ● running, ○ queued, – skipped or neutral),
     the name, and the duration or how long ago it finished.
   - **Failures first:** failed checks sort to the top of each group.
   - **Links:** each check's `detailsUrl` is clickable.
   - **Summary:** a line like "12 passed · 1 failed · 2 running".
4. **Description:** the PR body, rendered as readable markdown (headings, lists, code blocks, links).
   Glamour or an equivalent renderer goes here if it keeps link positions trackable for D10.
   Otherwise, a lightweight renderer.
5. **Conversation:** a single timeline sorted by time, made of:
   - issue comments,
   - reviews (state badge: Approved, Changes requested or Commented, plus the review body),
   - review threads:
     - Each thread shows its `file:line` label and a **Resolved** or **Unresolved** marker. No code or
       diff hunk is shown.
     - The comments inside a thread are nested under it.
     - Resolved threads are collapsed by default.
   - **Every timestamp is in local time** (system timezone), as both an absolute date ("Oct 6, 2:14
     PM") and a relative age ("3h ago").

**Diffs and file lists are never shown.**

### D12. Board interaction

- **Layout:**
  - Tabs at the top (Board, Review requests).
  - Lanes as columns, each with a header showing the lane name, tag color and count.
  - A status bar at the bottom (last refresh, changed count, errors, rate limit).
  - A help footer (`?` toggles the full help).
- **Lane width and scrolling:** lanes have a minimum width (~32 columns). When they don't all fit,
  the board scrolls horizontally and keeps the focused lane on screen. Each lane scrolls vertically on
  its own.
- **Card contents** (compact, 3–4 rows):
  - **Line 1:** `repo#123`, the activity dot, and a Draft/Merged/Closed badge.
  - **Line 2:** the title, truncated.
  - **Line 3:** status icons for checks (✓/✗/●), review decision, mergeability (⚠ conflict,
    ↓ behind), and the age since last update.
- **Sorting:** cards sort within a lane by most recently updated. Manual ordering inside a lane is
  out of scope.
- **Moving cards:** see the keys in §5. Drag-and-drop is rejected as fragile in a terminal and slower
  than a single keypress.

---

## 4. Architecture

```
cmd/lanes/main.go        entry point; flags (--version, --config)
internal/github/         go-gh GraphQL client, queries, mutations, response → domain mapping
internal/domain/         PR, CheckSummary, Review, Thread, Comment, MergeStatus (no UI, no I/O)
internal/store/          config.yaml + state.json load/save, atomic writes, schema version
internal/activity/       snapshot diff → "changed?" decision (pure, unit-tested)
internal/links/          URL extraction from markdown/plain text
internal/ui/             Bubble Tea models
    app.go               root model: tabs, refresh ticker, toasts, routing of messages
    board.go             lanes + cards, horizontal scroll
    reviews.go           review requests list
    modal.go             detail overlay, viewport, link hit-map
    picker.go            reusable list picker (move-to, link picker, merge method)
    tags.go              tag manager screen
    confirm.go           confirmation dialog
    theme.go             Lip Gloss styles, adaptive light/dark colors
```

- **Data flow:**
  1. A tick, an `r` press or a finished action sends a fetch command.
  2. The GitHub layer returns domain values as a message.
  3. The root model merges them with the store (tags, archived, snapshots) to build the lanes.
  4. Views render from that state.
- **Mutations** are commands that return a result message, which then triggers a single-PR refresh.
- **Pure logic** (activity diff, link extraction, lane assembly, merge-status sentences) lives
  outside `ui/` so it can be unit-tested without a terminal.
- **Tests:** `go test ./...` covering domain mapping (from recorded GraphQL fixtures), activity
  diffs, link extraction, lane assembly and store round-trips. UI models get Bubble Tea `teatest`
  smoke tests for key flows (move card, open modal, confirm merge with a fake client).

### GraphQL sketch

**List query.** One query per search: the board search, the review search, plus `nodes(ids:)` for
tracked closed PRs. Fields per PR:

- **Identity and state:** `id`, `number`, `title`, `url`, `isDraft`, `state`, `merged`, `closedAt`,
  `mergedAt`, `createdAt`, `updatedAt`, `author { login }`.
- **Branches:** `baseRefName`, `headRefName`.
- **Merge and review status:** `mergeable`, `mergeStateStatus`, `reviewDecision`,
  `autoMergeRequest { enabledAt mergeMethod }`, `viewerCanUpdate`.
- **Repository:** `nameWithOwner`, `mergeCommitAllowed`, `squashMergeAllowed`,
  `rebaseMergeAllowed`, `autoMergeAllowed`, `deleteBranchOnMerge`.
- **Checks:** `commits(last: 1) { nodes { commit { statusCheckRollup { state contexts(first: 100) {
  nodes { ... on CheckRun { name status conclusion startedAt completedAt detailsUrl
  checkSuite { workflowRun { workflow { name } } } } ... on StatusContext { context state targetUrl
  createdAt } } } } } } }`.
- **Activity counts:** `comments { totalCount }`, `reviews { totalCount }`, and
  `latestReviews(first: 20)` for the activity snapshot.

**Detail query** (one PR, when the modal opens): `body`,
`comments(first: 100) { author, body, createdAt, url }`,
`reviews(first: 100) { author, state, body, submittedAt }`, and
`reviewThreads(first: 100) { isResolved, path, line, comments(first: 50) { author, body, createdAt, url } }`.
Page through any connection that reports `hasNextPage`.

`mergeable` and `mergeStateStatus` can return UNKNOWN while GitHub computes them. Show "checking…"
and let the next refresh fill them in. Never treat UNKNOWN as a conflict.

---

## 5. Keybindings (v1)

| Key | Where | Action |
|---|---|---|
| `tab` / `shift+tab` | global | switch Board ↔ Review requests |
| `h` `l` / `←` `→` | board | focus previous/next lane |
| `j` `k` / `↓` `↑` | board, lists | select card |
| `H` `L` / `<` `>` | board | move card one lane left/right (retag) |
| `m` | board | "move to…" tag picker |
| `enter` | board, lists | open detail modal |
| `x` | board | archive card (merged/closed/any) |
| `t` | global | tag manager |
| `r` | global | refresh now |
| `o` | board, modal | link picker |
| `O` | board, modal | open PR in browser |
| `u` / `U` | board, modal | update branch (merge / rebase, rebase confirms) |
| `M` | board, modal | merge dialog (methods, delete-branch toggle, auto-merge) |
| `d` | board, modal | toggle draft ↔ ready for review |
| `esc` | modal, pickers | close |
| `?` | global | full help |
| `q` / `ctrl+c` | global | quit |

**Mouse:**
- Click a card to select it, and double-click to open it.
- Click a link in the modal to open it.
- Scroll the wheel over a lane or the modal to scroll it.
- Shift+drag to select text.

---

## 6. Files on disk

`~/.config/lanes/config.yaml` holds settings you edit, either in the app's tag manager or by hand:

```yaml
version: 1
refresh_interval: 60s
tags:
  - id: t_inprogress     # stable; assignments reference this, so renames are safe
    name: In Progress
    color: "#5B9CF6"
  - id: t_review
    name: Review
    color: "#C792EA"
  - id: t_testing
    name: Testing
    color: "#F5A623"
  - id: t_demo
    name: Demo
    color: "#3FB950"
  - id: t_done
    name: Done
    color: "#8B949E"
    terminal: true
```

`~/.config/lanes/state.json` is machine-written state that you shouldn't edit by hand:

```json
{
  "version": 1,
  "assignments": { "<PR node id>": "t_testing" },
  "archived": ["<PR node id>"],
  "snapshots": { "<PR node id>": { "seenAt": "…", "checks": "SUCCESS", "mergeable": "MERGEABLE",
                 "reviewDecision": "APPROVED", "comments": 4, "reviews": 2, "isDraft": false, "state": "OPEN" } }
}
```

- PRs are keyed by their **GraphQL node ID**, which stays stable through renames, transfers and
  title changes.
- Writes are atomic (write to a temp file, then rename).
- A `version` field allows future migrations.

---

## 7. Packaging files (written during the build, not published)

- `.goreleaser.yaml`:
  - builds `darwin/arm64`, `darwin/amd64`, `linux/arm64` and `linux/amd64`,
  - creates archives,
  - publishes a `brews` formula named `lanes` to `jehielmartinez/homebrew-tap`,
  - sets the formula's `dependencies`/`caveats` to say `gh auth login` is required.
- `.github/workflows/release.yml` runs GoReleaser on `v*` tags. It needs a `HOMEBREW_TAP_TOKEN`
  secret.
- `cli/gh-extension-precompile` produces the `gh` extension assets on the same tag, either in the
  same workflow or a second job.
- `README.md` covers install (both channels), first run, keys, and config.
- **License:** to be chosen at publish time (MIT is the default suggestion).

---

## 8. Out of scope for v1

- File diffs or file lists of any kind.
- Desktop notifications.
- Flags or extra tags beyond the single lane tag.
- Manual card ordering inside a lane.
- Drag-and-drop.
- Custom search queries or extra tabs.
- Writing comments, approving PRs or requesting reviewers from the tool.
- Syncing tags across machines.
- Re-running failed checks. This is a cheap candidate for v2 via `rerequestCheckSuite` or the REST
  re-run endpoint.

---

## 9. Build order (proposed)

1. `brew install go`, `go mod init github.com/jehielmartinez/gh-lanes`, skeleton, and a CI workflow
   running `go test ./...` and `go vet`.
2. GitHub layer: list query, domain mapping, and fixtures with tests.
3. Store: config and state load/save, tag CRUD, assignments, archive, and the snapshot diff with
   tests.
4. Board view: lanes, cards, focus and scroll, moving and retagging, refresh ticker, status bar.
5. Detail modal: sections, local time, markdown, link hit-map with mouse clicks, link picker.
6. Actions: update branch, merge dialog, auto-merge, draft/ready, open in browser.
7. Review requests tab.
8. Tag manager screen.
9. Packaging: GoReleaser, release workflow, README.
