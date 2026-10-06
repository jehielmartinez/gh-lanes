package domain

// UpdateMethod is how a branch is brought up to date with its base branch.
type UpdateMethod string

// The update methods GitHub supports.
const (
	UpdateMerge  UpdateMethod = "MERGE"
	UpdateRebase UpdateMethod = "REBASE"
)

// CanUpdateBranch reports whether the pull request's branch can be updated
// from its base branch by the viewer: it is open, the viewer may change it,
// and GitHub says the branch can be updated now.
func (pr PullRequest) CanUpdateBranch() bool {
	return pr.State == StateOpen && pr.ViewerCanUpdate && pr.ViewerCanUpdateBranch
}

// CanMarkReady reports whether the viewer can mark the pull request ready for
// review: it is an open draft they may change.
func (pr PullRequest) CanMarkReady() bool {
	return pr.State == StateOpen && pr.ViewerCanUpdate && pr.IsDraft
}

// CanConvertToDraft reports whether the viewer can turn the pull request back
// into a draft: it is open, not already a draft, and theirs to change.
func (pr PullRequest) CanConvertToDraft() bool {
	return pr.State == StateOpen && pr.ViewerCanUpdate && !pr.IsDraft
}
