package board

import (
	"slices"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// Missing returns, sorted, the IDs of tagged pull requests the open search
// didn't return. They have most likely merged or closed, and only a fetch by
// node ID can say.
func Missing(open []domain.PullRequest, assignments map[string]string) []string {
	seen := ids(open)
	var missing []string
	for id := range assignments {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	slices.Sort(missing)
	return missing
}

// Retain returns the pull requests the board shows after a refresh: every open
// one, plus each tagged one that has merged or closed. An untagged pull request
// drops off once it is no longer open, and so does a tagged one that is open
// but missing from the open search, as in an archived repository.
func Retain(open, tracked []domain.PullRequest, assignments map[string]string) []domain.PullRequest {
	prs := slices.Clone(open)
	for _, pr := range tracked {
		if _, tagged := assignments[pr.ID]; tagged && pr.Finished() {
			prs = append(prs, pr)
		}
	}
	return prs
}

// Archive returns copies of assignments and archived with the pull request
// taken off the board: untagged, and recorded as archived.
func Archive(assignments map[string]string, archived []domain.Archived, pr domain.PullRequest) (map[string]string, []domain.Archived) {
	next := slices.DeleteFunc(slices.Clone(archived), func(a domain.Archived) bool { return a.ID == pr.ID })
	next = append(next, domain.Archived{ID: pr.ID, Open: !pr.Finished()})
	return Assign(assignments, pr.ID, ""), next
}

// Reconcile updates the archived list against a fresh open search. A pull
// request missing from the search is no longer open; one that is back in it
// after being seen closed was reopened, so it leaves the archive and returns
// to Untagged. changed reports whether anything differs from archived.
func Reconcile(archived []domain.Archived, open []domain.PullRequest) (next []domain.Archived, changed bool) {
	seen := ids(open)
	for _, a := range archived {
		switch isOpen := seen[a.ID]; {
		case isOpen && !a.Open:
			changed = true
		case !isOpen && a.Open:
			changed = true
			next = append(next, domain.Archived{ID: a.ID})
		default:
			next = append(next, a)
		}
	}
	return next, changed
}

// Dimmed reports whether the pull request's card recedes in this lane:
// finished work in a terminal lane.
func (l Lane) Dimmed(pr domain.PullRequest) bool {
	return l.Tag.Terminal && pr.Finished()
}

func ids(prs []domain.PullRequest) map[string]bool {
	set := make(map[string]bool, len(prs))
	for _, pr := range prs {
		set[pr.ID] = true
	}
	return set
}
