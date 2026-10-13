package domain

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// Identity is who a reviewer is: a user's login or a team's owner/slug, and
// whether it is the login lanes runs as. Name is empty when GitHub won't say
// who it is.
type Identity struct {
	Name     string
	Team     bool
	IsViewer bool
}

// Reviewer is one row of a pull request's roll-call: someone asked to review
// it, someone who has, or both.
type Reviewer struct {
	Identity
	// Verdict is empty if the reviewer hasn't submitted a review.
	Verdict   ReviewState
	VerdictAt time.Time
	Awaiting  bool
}

// RollCall lists the pull request's reviewers, one per reviewer and never its
// author, those who decide the merge first and each group most recent first.
func (pr PullRequest) RollCall() []Reviewer {
	var rows []Reviewer
	index := map[string]int{}
	for _, r := range pr.LatestReviews {
		if verdictRank(r.State) < 0 || pr.isAuthor(r.Author) {
			continue
		}
		index[strings.ToLower(r.Author)] = len(rows)
		rows = append(rows, Reviewer{
			Identity: Identity{Name: r.Author, IsViewer: r.AuthorIsViewer},
			Verdict:  r.State, VerdictAt: r.SubmittedAt,
		})
	}
	for _, who := range pr.ReviewRequests {
		if pr.isAuthor(who.Name) {
			continue
		}
		if i, ok := index[strings.ToLower(who.Name)]; ok && who.Name != "" && !who.Team {
			rows[i].Awaiting = true
			continue
		}
		rows = append(rows, Reviewer{Identity: who, Awaiting: true})
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

// NobodyAskedToReview reports the stuck case: the base branch requires a
// review and the pull request has no reviewers at all.
func (pr PullRequest) NobodyAskedToReview() bool {
	return pr.ReviewDecision == ReviewRequired && len(pr.RollCall()) == 0
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
// as awaited whatever their earlier verdict, since they act next.
func (r Reviewer) rank() int {
	if r.Awaiting {
		return 1
	}
	return verdictRank(r.Verdict)
}

// verdictRank is a verdict's place in the roll-call, or -1 for a state that
// isn't a submitted review, such as the viewer's own unsubmitted draft.
func verdictRank(state ReviewState) int {
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
