# lanes

A terminal kanban board for the open pull requests you authored, with a separate queue for the pull requests whose review is requested of you.

## Language

### Views

**Board**:
The tab of lanes holding the pull requests you authored.
_Avoid_: Main view, kanban

**Review requests**:
The tab listing open pull requests whose review is requested of you. They never enter a lane.
_Avoid_: Review queue, reviews tab

**Archived**:
The tab listing archived pull requests, from either the Board or Review requests.

### Archiving

**Archive**:
Hiding a pull request from the tab it was on until you unarchive it. An archived pull request remembers which tab it came from.
_Avoid_: Dismiss, hide, delete

**Origin**:
The tab an archived pull request was archived from (Board or Review requests). Unarchiving returns it there, never anywhere else.

### Filtering

**Owner**:
The account that owns a pull request's repository: an organization, your own account, or someone else's personal account.
_Avoid_: Organization, org (when any account type is meant)

**Excluded**:
A repository whose pull requests are hidden from every tab. A repository is excluded if you unchecked it, or if you never chose for it and its owner is excluded.
_Avoid_: Disabled, muted, hidden

**Owner default**:
Whether an owner is excluded. It decides only for that owner's repositories you have never chosen for, including ones lanes has not seen yet. Your choice for a repository always overrides it.
