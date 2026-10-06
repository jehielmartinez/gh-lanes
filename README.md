# lanes

A terminal kanban board for the open pull requests you authored.

`lanes` finds every open PR you authored, across every org and repo, and shows them as lanes you
define yourself: In Progress, Review, Testing, Demo, Done, or your own. Checks, reviews,
mergeability and draft state refresh on their own. You can update a branch, merge, or switch
between draft and ready without leaving the terminal. A second tab lists the PRs where your review
is requested.

Your tags stay on your machine. `lanes` never writes labels, projects or comments to GitHub to
represent them.

It runs as `gh lanes` (a `gh` extension) or as a standalone `lanes` binary.

## Install

Both channels need the [GitHub CLI](https://cli.github.com/), because `lanes` uses its login.

In the commands below, `OWNER` is the account that publishes `lanes` releases.

### As a `gh` extension

```sh
gh extension install OWNER/gh-lanes
gh lanes
```

Upgrade with `gh extension upgrade lanes`.

### With Homebrew

```sh
brew install OWNER/tap/lanes
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
| `tab` / `shift+tab` | everywhere | switch between Board and Review requests |
| `h` `l` / `←` `→` | board | focus the previous or next lane |
| `j` `k` / `↓` `↑` | board, lists | select a card |
| `H` `L` / `<` `>` | board | move the card one lane left or right |
| `m` | board | move the card to any lane |
| `enter` | board, lists | open the PR's detail view |
| `x` | board | archive the card |
| `t` | everywhere | manage tags |
| `r` | everywhere | refresh now |
| `o` | board, detail | pick a link to open |
| `O` | board, detail | open the PR in the browser |
| `u` / `U` | board, detail | update the branch by merge / by rebase |
| `M` | board, detail | merge dialog |
| `d` | board, detail | toggle draft and ready for review |
| `esc` | detail, pickers | close |
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

- **The config file** (YAML) is yours to edit. It holds the refresh interval and your tags. The
  tag manager (`t`) writes it too, and `lanes` reads it again on every start.
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
