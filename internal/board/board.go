// Package board decides which lane each pull request sits in and in what order.
package board

import (
	"cmp"
	"maps"
	"slices"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// UntaggedName is the name of the lane that holds every pull request without a
// tag. It is always the first lane.
const UntaggedName = "Untagged"

// Lane is one column of the board.
type Lane struct {
	// Tag is the lane's tag. The Untagged lane has a tag with an empty ID.
	Tag          domain.Tag
	PullRequests []domain.PullRequest
}

// Untagged reports whether this is the Untagged lane.
func (l Lane) Untagged() bool { return l.Tag.ID == "" }

// Assemble builds the board's lanes: Untagged first, then one lane per tag in
// tag order, most recently updated first within each. assignments maps a pull
// request node ID to a tag ID; a pull request whose tag no longer exists is
// untagged, so every pull request is in exactly one lane. Archived pull
// requests are left off.
func Assemble(prs []domain.PullRequest, tags []domain.Tag, assignments map[string]string, archived []domain.Archived) []Lane {
	lanes := make([]Lane, 0, len(tags)+1)
	lanes = append(lanes, Lane{Tag: domain.Tag{Name: UntaggedName}})
	index := map[string]int{}
	for _, t := range tags {
		index[t.ID] = len(lanes)
		lanes = append(lanes, Lane{Tag: t})
	}

	hidden := map[string]bool{}
	for _, a := range archived {
		hidden[a.ID] = true
	}
	shown := slices.DeleteFunc(slices.Clone(prs), func(pr domain.PullRequest) bool { return hidden[pr.ID] })
	for _, pr := range ByUpdated(shown) {
		i := index[assignments[pr.ID]]
		lanes[i].PullRequests = append(lanes[i].PullRequests, pr)
	}
	return lanes
}

// ByUpdated returns a copy of prs, most recently updated first.
func ByUpdated(prs []domain.PullRequest) []domain.PullRequest {
	sorted := slices.Clone(prs)
	slices.SortStableFunc(sorted, func(a, b domain.PullRequest) int {
		return cmp.Compare(b.UpdatedAt.UnixNano(), a.UpdatedAt.UnixNano())
	})
	return sorted
}

// Assign returns a copy of assignments with the pull request moved to the tag.
// An empty tag ID moves it to Untagged.
func Assign(assignments map[string]string, prID, tagID string) map[string]string {
	next := maps.Clone(assignments)
	if next == nil {
		next = map[string]string{}
	}
	if tagID == "" {
		delete(next, prID)
	} else {
		next[prID] = tagID
	}
	return next
}

// Locate finds the pull request on the board: the index of its lane and of its
// card within that lane.
func Locate(lanes []Lane, prID string) (lane, card int, ok bool) {
	for li, l := range lanes {
		for ci, pr := range l.PullRequests {
			if pr.ID == prID {
				return li, ci, true
			}
		}
	}
	return 0, 0, false
}
