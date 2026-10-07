package board

import (
	"strings"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// Group is a run of cards in a lane or list that share a value for the
// grouping, shown under a header with its name and size.
type Group struct {
	Name string
	Size int
}

// Grouped returns copies of lanes with each lane's cards clustered into the
// groups of grouping. Groups are ordered by their most recently updated
// card, newest first, and cards keep that order inside a group. Group names
// compare case-insensitively and are spelled as on the group's most recently
// updated card. Each lane keeps exactly the cards it had.
func Grouped(lanes []Lane, grouping domain.Grouping) []Lane {
	grouped := make([]Lane, len(lanes))
	for i, lane := range lanes {
		lane.PullRequests, lane.Groups = GroupCards(lane.PullRequests, grouping)
		grouped[i] = lane
	}
	return grouped
}

// GroupCards returns prs ordered into the groups of grouping, and those
// groups, by the same rules as Grouped. With no grouping it returns prs as
// given and no groups.
func GroupCards(prs []domain.PullRequest, grouping domain.Grouping) ([]domain.PullRequest, []Group) {
	if grouping == domain.GroupingNone || len(prs) == 0 {
		return prs, nil
	}
	var keys []string
	names := map[string]string{}
	members := map[string][]domain.PullRequest{}
	for _, pr := range ByUpdated(prs) {
		name := groupName(pr, grouping)
		key := strings.ToLower(name)
		if _, seen := members[key]; !seen {
			keys = append(keys, key)
			names[key] = name
		}
		members[key] = append(members[key], pr)
	}
	ordered := make([]domain.PullRequest, 0, len(prs))
	groups := make([]Group, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, members[key]...)
		groups = append(groups, Group{Name: names[key], Size: len(members[key])})
	}
	return ordered, groups
}

func groupName(pr domain.PullRequest, grouping domain.Grouping) string {
	if grouping == domain.GroupingOwner {
		return pr.Repository.Owner()
	}
	return pr.Repository.NameWithOwner
}
