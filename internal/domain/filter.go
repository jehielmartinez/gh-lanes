package domain

// Filter says which repositories' pull requests are hidden from every tab.
// Owners and repositories are named as the config file names them, and are
// matched case-insensitively.
type Filter struct {
	// ExcludedOwners are the owners whose owner default is excluded.
	ExcludedOwners []string
	// Repositories maps a repository, as owner/name, to the choice made for
	// it. A repository with no entry follows its owner default.
	Repositories map[string]RepoChoice
}

// RepoChoice is the choice made for one repository, which always wins over
// its owner default.
type RepoChoice string

// The choices a repository can have.
const (
	RepoIncluded RepoChoice = "included"
	RepoExcluded RepoChoice = "excluded"
)
