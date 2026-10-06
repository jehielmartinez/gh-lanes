package board

import (
	"slices"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// Tagged returns the assignments whose tag still exists. A pull request
// assigned to a deleted tag is untagged, and is not kept once it closes.
func Tagged(assignments map[string]string, tags []domain.Tag) map[string]string {
	exists := map[string]bool{}
	for _, t := range tags {
		exists[t.ID] = true
	}
	live := map[string]string{}
	for pr, tag := range assignments {
		if exists[tag] {
			live[pr] = tag
		}
	}
	return live
}

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
// taken off the board: untagged, and recorded as archived along with the tag
// it had.
func Archive(assignments map[string]string, archived []domain.Archived, pr domain.PullRequest) (map[string]string, []domain.Archived) {
	next := slices.DeleteFunc(slices.Clone(archived), func(a domain.Archived) bool { return a.ID == pr.ID })
	next = append(next, domain.Archived{ID: pr.ID, Open: !pr.Finished(), Tag: assignments[pr.ID]})
	return Assign(assignments, pr.ID, ""), next
}

// Unarchive returns copies of assignments and archived with the pull request
// back on the board, in the tag it was archived from if that tag still
// exists, and Untagged otherwise. tagID is where it went; empty is Untagged.
func Unarchive(assignments map[string]string, archived []domain.Archived, tags []domain.Tag, prID string) (next map[string]string, rest []domain.Archived, tagID string) {
	for _, a := range archived {
		if a.ID == prID && slices.ContainsFunc(tags, func(t domain.Tag) bool { return t.ID == a.Tag }) {
			tagID = a.Tag
		}
	}
	rest = slices.DeleteFunc(slices.Clone(archived), func(a domain.Archived) bool { return a.ID == prID })
	return Assign(assignments, prID, tagID), rest, tagID
}

// ArchiveReviewRequest returns a copy of archived with the review request
// taken out of the review requests. It gets no lane, now or when unarchived.
func ArchiveReviewRequest(archived []domain.Archived, pr domain.PullRequest) []domain.Archived {
	next := slices.DeleteFunc(slices.Clone(archived), func(a domain.Archived) bool { return a.ID == pr.ID })
	return append(next, domain.Archived{ID: pr.ID, Open: true, Origin: domain.OriginReviewRequests})
}

// IsArchived reports whether the pull request is on the archived list.
func IsArchived(archived []domain.Archived, prID string) bool {
	return slices.ContainsFunc(archived, func(a domain.Archived) bool { return a.ID == prID })
}

// OriginOf is the tab the pull request was archived from.
func OriginOf(archived []domain.Archived, prID string) domain.Origin {
	for _, a := range archived {
		if a.ID == prID {
			return a.Origin
		}
	}
	return domain.OriginBoard
}

// Unarchived returns the pull requests that are not on the archived list.
func Unarchived(prs []domain.PullRequest, archived []domain.Archived) []domain.PullRequest {
	return slices.DeleteFunc(slices.Clone(prs), func(pr domain.PullRequest) bool { return IsArchived(archived, pr.ID) })
}

// Reconcile updates the archived list against a fresh open search and review
// requests search, checking each entry against its origin's. A pull request
// missing from the search has left it; one that is back after being seen gone
// was reopened or re-requested, so it leaves the archive and returns to its
// origin, Untagged for the board. changed reports whether anything differs
// from archived.
func Reconcile(archived []domain.Archived, open, requested []domain.PullRequest) (next []domain.Archived, changed bool) {
	seen := map[domain.Origin]map[string]bool{
		domain.OriginBoard:          ids(open),
		domain.OriginReviewRequests: ids(requested),
	}
	for _, a := range archived {
		switch isOpen := seen[a.Origin][a.ID]; {
		case isOpen && !a.Open:
			changed = true
		case !isOpen && a.Open:
			changed = true
			a.Open = false
			next = append(next, a)
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

// HasOpen reports whether any card on the board is an open pull request.
func HasOpen(lanes []Lane) bool {
	for _, l := range lanes {
		for _, pr := range l.PullRequests {
			if !pr.Finished() {
				return true
			}
		}
	}
	return false
}

func ids(prs []domain.PullRequest) map[string]bool {
	set := make(map[string]bool, len(prs))
	for _, pr := range prs {
		set[pr.ID] = true
	}
	return set
}
