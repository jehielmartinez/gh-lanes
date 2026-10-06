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

// MergeOffer is what the merge dialog can do for a pull request right now.
type MergeOffer int

// The merge offers.
const (
	// MergeOfferNone means GitHub would refuse any merge now.
	MergeOfferNone MergeOffer = iota
	MergeOfferMerge
	// MergeOfferAutoMerge stands in for a merge while checks or required
	// reviews are outstanding: GitHub merges once they are in.
	MergeOfferAutoMerge
	MergeOfferDisableAutoMerge
)

// MergeOffer decides what the merge dialog offers for the pull request.
// Mergeability GitHub is still computing is never treated as mergeable.
func (pr PullRequest) MergeOffer() MergeOffer {
	switch {
	case pr.State != StateOpen || !pr.ViewerCanUpdate:
		return MergeOfferNone
	case pr.AutoMerge != nil:
		return MergeOfferDisableAutoMerge
	case pr.IsDraft || len(pr.Repository.MergeMethods) == 0:
		return MergeOfferNone
	case pr.MergeStatus() == MergeStatusConflict:
		return MergeOfferNone
	case pr.Repository.AutoMergeAllowed && pr.awaitingRequirements():
		return MergeOfferAutoMerge
	case pr.MergeStatus() == MergeStatusChecking:
		return MergeOfferNone
	case pr.MergeStateStatus == MergeStateBlocked:
		// Nothing outstanding that auto-merge could wait for, so whatever
		// blocks it, such as a failed required check, needs a person.
		return MergeOfferNone
	}
	return MergeOfferMerge
}

// awaitingRequirements reports whether checks are still running or the
// reviews the base branch requires aren't all in.
func (pr PullRequest) awaitingRequirements() bool {
	if CountChecks(pr.Checks).Pending > 0 {
		return true
	}
	return pr.ReviewDecision == ReviewRequired || pr.ReviewDecision == ReviewChangesRequested
}

// DeletesBranchAfterMerge reports whether lanes must delete the head branch
// itself once the pull request merges, given whether the user asked for it.
// A repository that deletes merged branches does it without help.
func (pr PullRequest) DeletesBranchAfterMerge(wanted bool) bool {
	return wanted && !pr.Repository.DeleteBranchOnMerge && pr.HeadRefID != ""
}
