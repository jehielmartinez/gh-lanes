// Package board decides which lane each pull request sits in and in what order.
package board

import (
	"cmp"
	"slices"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// UntaggedName is the name of the lane that holds every pull request without a
// tag. It is always the first lane.
const UntaggedName = "Untagged"

// Lane is one column of the board.
type Lane struct {
	Name         string
	PullRequests []domain.PullRequest
}

// Assemble builds the board's lanes from the fetched pull requests, most
// recently updated first within each lane.
func Assemble(prs []domain.PullRequest) []Lane {
	sorted := slices.Clone(prs)
	slices.SortStableFunc(sorted, func(a, b domain.PullRequest) int {
		return cmp.Compare(b.UpdatedAt.UnixNano(), a.UpdatedAt.UnixNano())
	})
	return []Lane{{Name: UntaggedName, PullRequests: sorted}}
}
