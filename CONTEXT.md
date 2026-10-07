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

### Grouping

**Grouping**:
The one global mode that clusters cards into groups on every tab: None, Owner, Repository or Title pattern. Only one applies at a time. None shows no groups. Grouping never changes a pull request's lane.
_Avoid_: Swimlanes, sorting

**Group**:
A cluster of cards under a header inside a lane, or inside the Review requests or Archived list, that share the same value for the current grouping. Group names compare case-insensitively.
_Avoid_: Section (that names a part of the detail modal), sub-lane, swimlane

**Title pattern**:
The one regular expression that Title pattern grouping matches against pull request titles. The first match in a title names the pull request's group, so each pull request is in exactly one group.
_Avoid_: Filter, key pattern

**No match**:
The group holding pull requests whose title doesn't match the title pattern. It always comes last.
