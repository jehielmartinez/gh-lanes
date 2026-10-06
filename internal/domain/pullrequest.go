// Package domain holds the values the rest of lanes reasons about. It does no
// I/O and imports nothing from this module.
package domain

import "time"

// PullRequest is one pull request as the board sees it.
type PullRequest struct {
	// ID is the GraphQL node ID, the key for everything stored locally.
	ID         string
	Number     int
	Title      string
	URL        string
	Author     string
	State      State
	IsDraft    bool
	Repository Repository

	CreatedAt time.Time
	UpdatedAt time.Time
	// MergedAt and ClosedAt are zero while the pull request is open.
	MergedAt time.Time
	ClosedAt time.Time

	BaseRef string
	HeadRef string

	Mergeable        Mergeable
	MergeStateStatus MergeStateStatus
	ReviewDecision   ReviewDecision
	// AutoMerge is nil when auto-merge is not enabled.
	AutoMerge       *AutoMerge
	ViewerCanUpdate bool

	Checks        []Check
	CommentCount  int
	ReviewCount   int
	LatestReviews []Review
}

// State is whether a pull request is open, merged or closed without merging.
type State string

// The states GitHub reports.
const (
	StateOpen   State = "OPEN"
	StateClosed State = "CLOSED"
	StateMerged State = "MERGED"
)

// Repository is the repository a pull request belongs to, with the merge
// settings that decide which actions it offers.
type Repository struct {
	NameWithOwner       string
	MergeMethods        []MergeMethod
	AutoMergeAllowed    bool
	DeleteBranchOnMerge bool
}

// MergeMethod is one way GitHub can merge a pull request.
type MergeMethod string

// The merge methods GitHub supports.
const (
	MergeMethodMerge  MergeMethod = "MERGE"
	MergeMethodSquash MergeMethod = "SQUASH"
	MergeMethodRebase MergeMethod = "REBASE"
)

// AutoMerge describes a pending auto-merge request.
type AutoMerge struct {
	Method    MergeMethod
	EnabledBy string
	EnabledAt time.Time
}

// Mergeable is GitHub's verdict on whether the branch merges cleanly.
type Mergeable string

// The mergeable values GitHub reports. MergeableUnknown means GitHub is still
// computing it.
const (
	MergeableMergeable   Mergeable = "MERGEABLE"
	MergeableConflicting Mergeable = "CONFLICTING"
	MergeableUnknown     Mergeable = "UNKNOWN"
)

// MergeStateStatus is GitHub's mergeStateStatus enum, kept as reported.
type MergeStateStatus string

// The merge state statuses GitHub reports.
const (
	MergeStateBehind   MergeStateStatus = "BEHIND"
	MergeStateBlocked  MergeStateStatus = "BLOCKED"
	MergeStateClean    MergeStateStatus = "CLEAN"
	MergeStateDirty    MergeStateStatus = "DIRTY"
	MergeStateDraft    MergeStateStatus = "DRAFT"
	MergeStateHasHooks MergeStateStatus = "HAS_HOOKS"
	MergeStateUnknown  MergeStateStatus = "UNKNOWN"
	MergeStateUnstable MergeStateStatus = "UNSTABLE"
)

// ReviewDecision is the review outcome required by the base branch's rules.
// It is empty when the repository requires no review.
type ReviewDecision string

// The review decisions GitHub reports.
const (
	ReviewApproved         ReviewDecision = "APPROVED"
	ReviewChangesRequested ReviewDecision = "CHANGES_REQUESTED"
	ReviewRequired         ReviewDecision = "REVIEW_REQUIRED"
)

// Review is the latest review one reviewer left.
type Review struct {
	Author      string
	State       string
	SubmittedAt time.Time
}

// MergeStatus is the one-glance answer to "can this merge as it stands?".
type MergeStatus int

// The merge statuses, from nothing-to-report to blocked.
const (
	MergeStatusClear MergeStatus = iota
	// MergeStatusChecking means GitHub hasn't finished computing
	// mergeability. It is neither a conflict nor mergeable.
	MergeStatusChecking
	MergeStatusBehind
	MergeStatusConflict
)

// MergeStatus decides the pull request's merge status from mergeable and
// mergeStateStatus.
func (pr PullRequest) MergeStatus() MergeStatus {
	switch {
	case pr.Mergeable == MergeableUnknown:
		return MergeStatusChecking
	case pr.Mergeable == MergeableConflicting || pr.MergeStateStatus == MergeStateDirty:
		return MergeStatusConflict
	case pr.MergeStateStatus == MergeStateUnknown:
		return MergeStatusChecking
	case pr.MergeStateStatus == MergeStateBehind:
		return MergeStatusBehind
	}
	return MergeStatusClear
}

// MergeSentence explains in plain words whether the pull request can merge
// into its base branch, and if not, why.
func (pr PullRequest) MergeSentence() string {
	base := pr.BaseRef
	switch pr.State {
	case StateMerged:
		return "Merged into " + base + "."
	case StateClosed:
		return "Closed without merging into " + base + "."
	}
	switch pr.MergeStatus() {
	case MergeStatusChecking:
		return "Checking whether this can merge into " + base + "…"
	case MergeStatusConflict:
		return "Conflicts with " + base + ". Resolve them before merging."
	case MergeStatusBehind:
		if pr.ViewerCanUpdate {
			return "Behind " + base + ". Update branch available."
		}
		return "Behind " + base + ". The branch needs updating before it can merge."
	}
	switch pr.MergeStateStatus {
	case MergeStateBlocked:
		return "Blocked from merging into " + base + " until required reviews or checks pass."
	case MergeStateDraft:
		return "Draft. Mark it ready for review before merging into " + base + "."
	case MergeStateUnstable:
		return "Can merge into " + base + ", though some checks haven't passed."
	case MergeStateHasHooks:
		return "Ready to merge into " + base + ". Its pre-receive hooks will run."
	case MergeStateClean:
		return "Ready to merge into " + base + "."
	}
	return "No conflicts with " + base + "."
}
