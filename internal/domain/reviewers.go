package domain

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// ReviewRequest is a review asked of a user or team and not yet submitted.
type ReviewRequest struct {
	// Reviewer is the user's login or the team's owner/slug. It is empty when
	// GitHub won't say who it is, as it does for a team the login can't see.
	Reviewer string
	Team     bool
	// ForViewer is set when the review is asked of the login lanes runs as.
	ForViewer bool
}

// Reviewer is one row of a pull request's roll-call: someone asked to review
// it, someone who has, or both.
type Reviewer struct {
	// Name is the user's login or the team's owner/slug, empty when GitHub
	// won't say who it is.
	Name     string
	Team     bool
	IsViewer bool
	// Verdict is the state of the reviewer's latest review, empty if they
	// haven't submitted one.
	Verdict   string
	VerdictAt time.Time
	Awaiting  bool
}

// RollCall lists the pull request's reviewers, one per reviewer, leaving out
// its author. Changes requested come first, then those awaiting review,
// approvals, comments and dismissals, each most recent first.
func (pr PullRequest) RollCall() []Reviewer {
	var rows []Reviewer
	index := map[string]int{}
	for _, r := range pr.LatestReviews {
		if verdictRank(r.State) < 0 || pr.isAuthor(r.Author) {
			continue
		}
		index[strings.ToLower(r.Author)] = len(rows)
		rows = append(rows, Reviewer{Name: r.Author, IsViewer: r.ByViewer, Verdict: r.State, VerdictAt: r.SubmittedAt})
	}
	for _, q := range pr.ReviewRequests {
		if pr.isAuthor(q.Reviewer) {
			continue
		}
		if i, ok := index[strings.ToLower(q.Reviewer)]; ok && q.Reviewer != "" && !q.Team {
			rows[i].Awaiting = true
			continue
		}
		rows = append(rows, Reviewer{Name: q.Reviewer, Team: q.Team, IsViewer: q.ForViewer, Awaiting: true})
	}
	slices.SortStableFunc(rows, func(a, b Reviewer) int {
		return cmp.Or(
			cmp.Compare(a.rank(), b.rank()),
			b.VerdictAt.Compare(a.VerdictAt),
			cmp.Compare(a.unnamed(), b.unnamed()),
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
		)
	})
	return rows
}

// unnamed sorts someone GitHub won't name after everyone it will.
func (r Reviewer) unnamed() int {
	if r.Name == "" {
		return 1
	}
	return 0
}

func (pr PullRequest) isAuthor(login string) bool {
	return login != "" && strings.EqualFold(login, pr.Author)
}

// rank orders a row by what it means for the merge: a reviewer awaited sorts
// as awaited whatever their earlier verdict, since that is who acts next.
func (r Reviewer) rank() int {
	if r.Awaiting {
		return 1
	}
	return verdictRank(r.Verdict)
}

// verdictRank is a verdict's place in the roll-call, or -1 for a state that
// isn't a submitted review, such as the viewer's own unsubmitted draft.
func verdictRank(state string) int {
	switch state {
	case ReviewStateChangesRequested:
		return 0
	case ReviewStateApproved:
		return 2
	case ReviewStateCommented:
		return 3
	case ReviewStateDismissed:
		return 4
	}
	return -1
}
