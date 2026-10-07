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
	return OwnerExcluded(f, ownerOf(nameWithOwner))
}

// OwnerExcluded reports whether the owner's owner default is excluded.
func OwnerExcluded(f domain.Filter, owner string) bool {
	return slices.ContainsFunc(f.ExcludedOwners, func(o string) bool { return strings.EqualFold(o, owner) })
}

// ToggleOwner flips the owner's owner default. Including an owner drops every
// spelling of it from the excluded owners, so the filter holds only
// exclusions.
func ToggleOwner(f domain.Filter, owner string) domain.Filter {
	if OwnerExcluded(f, owner) {
		f.ExcludedOwners = slices.DeleteFunc(slices.Clone(f.ExcludedOwners), func(o string) bool { return strings.EqualFold(o, owner) })
	} else {
		f.ExcludedOwners = append(slices.Clone(f.ExcludedOwners), owner)
	}
	return f
}

// Owner is one owner the filter screen lists.
type Owner struct {
	Login string
	// PullRequests counts the owner's pull requests among those the screen
	// was given, whether the filter hides them or not.
	PullRequests int
	Excluded     bool
}

// Owners lists the viewer's own account, the viewer's organizations, the
// owner of every pull request in prs and every owner the filter names, each
// once whatever its capitalisation. The viewer comes first, when known; the
// rest sort by pull request count, most first, then by name.
func Owners(viewer string, organizations []string, prs []domain.PullRequest, f domain.Filter) []Owner {
	var owners []Owner
	index := map[string]int{}
	add := func(login string, prs int) {
		key := strings.ToLower(login)
		i, ok := index[key]
		if !ok {
			i = len(owners)
			index[key] = i
			owners = append(owners, Owner{Login: login, Excluded: OwnerExcluded(f, login)})
		}
		owners[i].PullRequests += prs
	}
	// GitHub's spelling of a login is added before the config's, so it is
	// the one shown.
	if viewer != "" {
		add(viewer, 0)
	}
	for _, o := range organizations {
		add(o, 0)
	}
	for _, pr := range prs {
		add(ownerOf(pr.Repository.NameWithOwner), 1)
	}
	for _, o := range f.ExcludedOwners {
		add(o, 0)
	}
	for repo := range f.Repositories {
		add(ownerOf(repo), 0)
	}
	slices.SortStableFunc(owners, func(a, b Owner) int {
		switch {
		case viewer != "" && strings.EqualFold(a.Login, viewer):
			return -1
		case viewer != "" && strings.EqualFold(b.Login, viewer):
			return 1
		case a.PullRequests != b.PullRequests:
			return b.PullRequests - a.PullRequests
		}
		return strings.Compare(strings.ToLower(a.Login), strings.ToLower(b.Login))
	})
	return owners
}

func ownerOf(nameWithOwner string) string {
	owner, _, _ := strings.Cut(nameWithOwner, "/")
	return owner
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
