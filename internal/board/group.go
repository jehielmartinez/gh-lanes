package board

import (
	"regexp"
	"strings"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// Group is a run of cards in a lane or list that share a value for the
// grouping, shown under a header with its name and size.
type Group struct {
	Name string
	Size int
	// NoMatch marks the group of cards whose title the title pattern doesn't
	// match. It has no name of its own.
	NoMatch bool
}

// Grouped returns copies of lanes with each lane's cards clustered into the
// groups of grouping, matching titlePattern against titles when grouping is
// by title pattern. Groups are ordered by their most recently updated card,
// newest first, with No match last, and cards keep that order inside a
// group. Group names compare case-insensitively and are spelled as on the
// group's most recently updated card. Each lane keeps exactly the cards it
// had.
func Grouped(lanes []Lane, grouping domain.Grouping, titlePattern *regexp.Regexp) []Lane {
	grouped := make([]Lane, len(lanes))
	for i, lane := range lanes {
		lane.PullRequests, lane.Groups = GroupCards(lane.PullRequests, grouping, titlePattern)
		grouped[i] = lane
	}
	return grouped
}

// GroupCards returns prs ordered into the groups of grouping, and those
// groups, by the same rules as Grouped. With no grouping it returns prs as
// given and no groups.
func GroupCards(prs []domain.PullRequest, grouping domain.Grouping, titlePattern *regexp.Regexp) ([]domain.PullRequest, []Group) {
	if grouping == domain.GroupingNone || len(prs) == 0 {
		return prs, nil
	}
	var keys []string
	names := map[string]string{}
	members := map[string][]domain.PullRequest{}
	var unmatched []domain.PullRequest
	for _, pr := range ByUpdated(prs) {
		name, ok := groupName(pr, grouping, titlePattern)
		if !ok {
			unmatched = append(unmatched, pr)
			continue
		}
		key := strings.ToLower(name)
		if _, seen := members[key]; !seen {
			keys = append(keys, key)
			names[key] = name
		}
		members[key] = append(members[key], pr)
	}
	ordered := make([]domain.PullRequest, 0, len(prs))
	groups := make([]Group, 0, len(keys)+1)
	for _, key := range keys {
		ordered = append(ordered, members[key]...)
		groups = append(groups, Group{Name: names[key], Size: len(members[key])})
	}
	if len(unmatched) > 0 {
		ordered = append(ordered, unmatched...)
		groups = append(groups, Group{Size: len(unmatched), NoMatch: true})
	}
	return ordered, groups
}

// groupName names pr's group, or reports that its title doesn't match the
// title pattern. The title is already sanitized, so a name taken from it
// carries no escape sequences.
func groupName(pr domain.PullRequest, grouping domain.Grouping, titlePattern *regexp.Regexp) (string, bool) {
	switch grouping {
	case domain.GroupingOwner:
		return pr.Repository.Owner(), true
	case domain.GroupingTitlePattern:
		// An empty match names nothing a header could show, so it counts as
		// no match.
		name := titlePattern.FindString(pr.Title)
		return name, name != ""
	}
	return pr.Repository.NameWithOwner, true
}
