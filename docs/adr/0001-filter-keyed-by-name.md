# The filter is keyed by owner and repository name, not node ID

Pull requests are keyed by node ID so that renames and transfers keep their tag. The owner and repository filter is the exception: the config file stores names like `octo-org` and `octo-org/sample-repo`. The filter is a setting you read and edit by hand in YAML, and node IDs would make that impractical. When a filtered repository is renamed or transferred, its choice stops matching. The repository then falls back to its owner default, and the old name stays listed with no pull requests until you clear it. That failure is safe because nothing is lost: at worst a hidden repository shows up again. A lost tag assignment, by contrast, can't be recovered, which is why pull requests keep node IDs.

## Considered Options

- **Node ID, with the name kept alongside for display.** Survives renames, but nobody can hand-edit the file, and lanes would have to resolve IDs to show the list. Rejected.
