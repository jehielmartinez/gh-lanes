package board

import (
	"slices"
	"strings"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// Excluded reports whether the filter hides the pull requests of the
// repository named owner/name: by the choice made for it, or, with none, by
// its owner default.
func Excluded(f domain.Filter, nameWithOwner string) bool {
	for repo, choice := range f.Repositories {
		if strings.EqualFold(repo, nameWithOwner) {
			return choice == domain.RepoExcluded
		}
	}
	owner, _, _ := strings.Cut(nameWithOwner, "/")
	return slices.ContainsFunc(f.ExcludedOwners, func(o string) bool { return strings.EqualFold(o, owner) })
}

// Visible returns the pull requests whose repository the filter doesn't
// exclude.
func Visible(prs []domain.PullRequest, f domain.Filter) []domain.PullRequest {
	return slices.DeleteFunc(slices.Clone(prs), func(pr domain.PullRequest) bool {
		return Excluded(f, pr.Repository.NameWithOwner)
	})
}

// ArchivedShown counts the archived pull requests the filter doesn't hide.
// seen are the pull requests whose repository is known; an archived one not
// among them can't be judged until it is fetched, so it counts as shown.
func ArchivedShown(archived []domain.Archived, seen []domain.PullRequest, f domain.Filter) int {
	hidden := map[string]bool{}
	for _, pr := range seen {
		if Excluded(f, pr.Repository.NameWithOwner) {
			hidden[pr.ID] = true
		}
	}
	shown := 0
	for _, a := range archived {
		if !hidden[a.ID] {
			shown++
		}
	}
	return shown
}
