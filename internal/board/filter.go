package board

import (
	"maps"
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

// OwnerPartial reports whether any repository of the owner has a choice that
// differs from its owner default, so its repositories don't all share the
// owner's check.
func OwnerPartial(f domain.Filter, owner string) bool {
	excluded := OwnerExcluded(f, owner)
	for repo, choice := range f.Repositories {
		if strings.EqualFold(ownerOf(repo), owner) && (choice == domain.RepoExcluded) != excluded {
			return true
		}
	}
	return false
}

// ToggleOwner flips the owner's owner default, or includes a partial owner,
// and clears the choice of every repository under it so they all match.
// Including an owner drops every spelling of it from the excluded owners, so
// the filter holds only exclusions.
func ToggleOwner(f domain.Filter, owner string) domain.Filter {
	include := OwnerExcluded(f, owner) || OwnerPartial(f, owner)
	f.ExcludedOwners = slices.DeleteFunc(slices.Clone(f.ExcludedOwners), func(o string) bool { return strings.EqualFold(o, owner) })
	if !include {
		f.ExcludedOwners = append(f.ExcludedOwners, owner)
	}
	repos := maps.Clone(f.Repositories)
	maps.DeleteFunc(repos, func(repo string, _ domain.RepoChoice) bool { return strings.EqualFold(ownerOf(repo), owner) })
	f.Repositories = repos
	return pruned(f)
}

// ToggleRepository flips the check of the repository named owner/name. Its
// choice is kept only where it differs from the owner default, so the filter
// holds only exceptions.
func ToggleRepository(f domain.Filter, nameWithOwner string) domain.Filter {
	exclude := !Excluded(f, nameWithOwner)
	repos := maps.Clone(f.Repositories)
	maps.DeleteFunc(repos, func(repo string, _ domain.RepoChoice) bool { return strings.EqualFold(repo, nameWithOwner) })
	if exclude != OwnerExcluded(f, ownerOf(nameWithOwner)) {
		if repos == nil {
			repos = map[string]domain.RepoChoice{}
		}
		choice := domain.RepoIncluded
		if exclude {
			choice = domain.RepoExcluded
		}
		repos[nameWithOwner] = choice
	}
	f.Repositories = repos
	return pruned(f)
}

// pruned drops every repository choice that matches its owner default, such
// as one left behind by a hand edit, so the filter holds only exceptions.
func pruned(f domain.Filter) domain.Filter {
	repos := maps.Clone(f.Repositories)
	maps.DeleteFunc(repos, func(repo string, choice domain.RepoChoice) bool {
		return (choice == domain.RepoExcluded) == OwnerExcluded(f, ownerOf(repo))
	})
	f.Repositories = repos
	return f
}

// Owner is one owner the filter screen lists.
type Owner struct {
	Login string
	// PullRequests counts the owner's pull requests among those the screen
	// was given, whether the filter hides them or not.
	PullRequests int
	// Excluded is the owner default.
	Excluded bool
	Partial  bool
	// Repositories are the owner's known repositories, sorted by name.
	Repositories []Repository
}

// Repository is one repository the filter screen lists under its owner.
type Repository struct {
	NameWithOwner string
	// PullRequests counts as Owner.PullRequests does.
	PullRequests int
	Excluded     bool
}

// Name is the repository's name without its owner.
func (r Repository) Name() string {
	_, name, _ := strings.Cut(r.NameWithOwner, "/")
	return name
}

// Owners lists the viewer's own account, the owner of every pull request in
// prs and every owner the filter names, each once whatever its
// capitalisation, with the repositories of those pull requests and those the
// filter names. The viewer comes first, when known; the rest sort by pull
// request count, most first, then by name.
func Owners(viewer string, prs []domain.PullRequest, f domain.Filter) []Owner {
	var owners []Owner
	index := map[string]int{}
	add := func(login string, prs int) int {
		key := strings.ToLower(login)
		i, ok := index[key]
		if !ok {
			i = len(owners)
			index[key] = i
			owners = append(owners, Owner{Login: login, Excluded: OwnerExcluded(f, login), Partial: OwnerPartial(f, login)})
		}
		owners[i].PullRequests += prs
		return i
	}
	repoIndex := map[string]int{}
	addRepo := func(nameWithOwner string, prs int) {
		o := add(ownerOf(nameWithOwner), prs)
		key := strings.ToLower(nameWithOwner)
		i, ok := repoIndex[key]
		if !ok {
			i = len(owners[o].Repositories)
			repoIndex[key] = i
			owners[o].Repositories = append(owners[o].Repositories, Repository{NameWithOwner: nameWithOwner, Excluded: Excluded(f, nameWithOwner)})
		}
		owners[o].Repositories[i].PullRequests += prs
	}
	// GitHub's spelling of a login or repository is added before the
	// config's, so it is the one shown.
	if viewer != "" {
		add(viewer, 0)
	}
	for _, pr := range prs {
		addRepo(pr.Repository.NameWithOwner, 1)
	}
	for _, o := range f.ExcludedOwners {
		add(o, 0)
	}
	for _, repo := range slices.Sorted(maps.Keys(f.Repositories)) {
		addRepo(repo, 0)
	}
	for i := range owners {
		slices.SortStableFunc(owners[i].Repositories, func(a, b Repository) int {
			return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
		})
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
