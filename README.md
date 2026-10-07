# lanes

A terminal kanban board for the open pull requests you authored.

`lanes` finds every open PR you authored, across every org and repo, and shows them as lanes you
define yourself: In Progress, Review, Testing, Demo, Done, or your own. Checks, reviews,
mergeability and draft state refresh on their own. You can update a branch, merge, or switch
between draft and ready without leaving the terminal. A second tab lists the PRs where your review
is requested, and a third lists the PRs you archived off the board.

Your tags stay on your machine. `lanes` never writes labels, projects or comments to GitHub to
represent them.

It runs as `gh lanes` (a `gh` extension) or as a standalone `lanes` binary.

## Install

Both channels need the [GitHub CLI](https://cli.github.com/), because `lanes` uses its login.

### As a `gh` extension

```sh
gh extension install jehielmartinez/gh-lanes
gh lanes
```

Upgrade with `gh extension upgrade lanes`.

### With Homebrew

```sh
brew install jehielmartinez/tap/lanes
lanes
```

Upgrade with `brew upgrade lanes`.

### Platforms

Release builds cover macOS and Linux, on arm64 and amd64.

## First run

1. Sign in to GitHub with the CLI, if you haven't already:

   ```sh
   gh auth login
   ```

   `lanes` has no login of its own. It reads the account, token and host from `gh`, and honours
   `GH_TOKEN` and `GH_HOST` the same way `gh` does. With no login, it exits and tells you to run
   `gh auth login`.

2. Start it with `gh lanes` or `lanes`.

On the first run, `lanes` writes a config file with the default tags In Progress, Review, Testing,
Demo and Done (Done is terminal). Every open PR you authored lands in the **Untagged** lane, which
is always first and works as your inbox. Move cards into lanes as you go.

The first run is quiet: no card is marked as changed. After that, a dot marks cards with activity
you haven't seen, and a "new" marker flags PRs that appeared since.

### Flags

| Flag | Effect |
|---|---|
| `--version` | Print the version and exit. |
| `--config DIR` | Use `DIR` as the config directory. |

## Keybindings

Press `?` in the app for the full help screen. It is generated from the app's keymap, so it is
always current.

| Key | Where | Action |
|---|---|---|
| `tab` / `shift+tab` | everywhere | switch between Board, Review requests and Archived |
| `h` `l` / `←` `→` | board | focus the previous or next lane |
| `j` `k` / `↓` `↑` | board, lists | select a card |
| `H` `L` / `<` `>` | board | move the card one lane left or right |
| `m` | board | move the card to any lane |
| `enter` | board, lists | open the PR's detail view |
| `x` | board | archive the card |
| `x` | archived | unarchive: put the card back in the lane it was archived from |
| `t` | everywhere | manage tags |
| `f` | board, lists | filter: `space` hides or shows an owner's or a repository's PRs everywhere, `enter` `l` `h` expand or collapse an owner, `a` loads all of an owner's repositories |
| `r` | everywhere | refresh now |
| `o` | board, detail | pick a link to open |
| `O` | board, detail | open the PR in the browser |
| `u` / `U` | board, detail | update the branch by merge / by rebase |
| `M` | board, detail | merge dialog |
| `d` | board, detail | toggle draft and ready for review |
| `1`–`4` | detail | open or close Status, Checks, Description, Conversation (or click the heading) |
| `e` | detail | show or hide resolved review threads |
| `esc` | detail, pickers, filter | close |
| `?` | everywhere | full help |
| `q` / `ctrl+c` | everywhere | quit |

Merge, update by rebase and convert to draft always ask for confirmation first. Update by merge
and ready for review run immediately. Actions appear only when GitHub allows them for that PR, and
the merge dialog lists only the merge methods the repo allows.

### Mouse

Click a card to select it, and double-click to open it. In the detail view, click any underlined
link to open it in your browser. Hold Shift while dragging to select text.

## Config

`lanes` keeps its files in `$XDG_CONFIG_HOME/lanes`. If `XDG_CONFIG_HOME` isn't set, it uses
`~/.config/lanes`. That is the same on every OS, macOS included. `--config` overrides both.

The directory holds two files:

- **The config file** (YAML) is yours to edit. It holds the refresh interval, your tags and the filter. The
  tag manager (`t`) and the filter screen (`f`) write it too, and `lanes` reads it again on every start.
- **The state file** (JSON) is written by the app. It holds which PR is in which lane, the
  archived PRs, and what you last saw of each PR. Don't edit it by hand.

A config file looks like this:

```yaml
version: 1
refresh_interval: 60s
tags:
  - id: in-progress
    name: In Progress
    color: "#3B82F6"
  - id: review
    name: Review
    color: "#F59E0B"
  - id: done
    name: Done
    color: "#10B981"
    terminal: true
filter:
  excluded_owners:
    - octo-org
  repositories:
    octo-org/sample-repo: included
    user-a/noisy-repo: excluded
```

| Field | Meaning |
|---|---|
| `version` | The file's schema version. `lanes` uses it to migrate the file after an upgrade. Leave it as it is. |
| `refresh_interval` | How often the board refreshes. The default is `60s`. A longer interval uses less of your GitHub API budget. |
| `tags` | Your lanes, left to right. The Untagged lane isn't listed: it is always first and can't be renamed or deleted. |
| `tags[].id` | A stable ID. Lane assignments point at it, so renaming a tag keeps its PRs. Don't change it. |
| `tags[].name` | The lane's name. |
| `tags[].color` | The lane's color. |
| `tags[].terminal` | Optional. Marks a lane like Done: merged and closed PRs in it are dimmed. |
| `filter` | Optional. Hides PRs by owner and repository on Board, Review requests and Archived. Without it, everything is shown. |
| `filter.excluded_owners` | Owners (orgs or users) whose repositories are hidden, including repositories `lanes` hasn't seen yet. |
| `filter.repositories` | Per-repository choices, as `owner/name: included` or `owner/name: excluded`. A repository's choice always wins over its owner. |

Filter names match whatever their capitalisation. Hiding a PR keeps its lane, archive state and
activity: when it is shown again, it is where you left it, marked if it changed meanwhile. A
renamed or transferred repository falls back to its owner. `lanes` reads the filter on start, so
restart it after editing by hand.

The filter screen (`f`) lists your own account first, then every org you belong to and every owner
with an open PR or a stored choice, most PRs first. An org with no open PRs is listed too, so you can
hide it before it causes noise. Under each owner are its repositories with an open PR or a stored
choice, sorted by name; `enter`, `l` and `h` expand and collapse it. Each row counts its open PRs on
Board and Review requests, hidden ones included. `space` checks or unchecks the selected row, which
applies straight away and saves the config. A repository's check always wins over its owner's, and
an owner whose repositories are mixed shows as `[~]` and starts expanded. Checking or unchecking an
owner sets all its repositories to match, so the config keeps only the exceptions. Opening the
screen asks GitHub which account you are signed in as and which orgs you belong to; an org that
enforces SAML SSO may be missing until you authorise `gh` for it. `a` on an owner, or on one of its
repositories, loads every repository that owner has from GitHub, so you can hide one before you have
a PR in it; loaded repositories follow the owner's check unless you have chosen for them. A long list
scrolls with the selection. If a request fails, the error shows at the foot of the screen and the
other rows still work.

Deleting a tag moves its PRs to Untagged.

Neither file ever holds a token. Authentication stays in `gh`'s own storage.

## Security

`lanes` treats PR content as untrusted. It strips terminal escape sequences from titles,
descriptions and comments before display. It opens only `http` and `https` links, and passes them
to the OS opener (`open` on macOS, `xdg-open` on Linux) as a single argument, never through a
shell.

## Releasing

Pushing a `v*` tag runs `.github/workflows/release.yml`. That workflow:

1. runs GoReleaser (`.goreleaser.yaml`), which builds the `lanes` binaries, creates the GitHub
   release with the archives, and pushes the `lanes` Homebrew cask to the `homebrew-tap` repo of
   the account that owns this one;
2. runs `cli/gh-extension-precompile`, which adds the `gh lanes` extension binaries to the same
   release.

The workflow needs a repository secret named `HOMEBREW_TAP_TOKEN`, holding a token that can push to
the tap repo. Creating the tap repo, that secret and the tag are manual steps.

To try a release build locally without publishing anything:

```sh
HOMEBREW_TAP_OWNER=octo-org HOMEBREW_TAP_TOKEN=unused goreleaser release --snapshot --clean
```
