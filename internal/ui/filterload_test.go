package ui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// The cursor GitHub hands back after the first page of octo-org's
// repositories.
const ownerReposPage2 = "Y3Vyc29yOjU="

// replyOwnerRepositories answers the repositories query for octo-org with its
// two pages of fixtures.
func replyOwnerRepositories(t *testing.T, transport *githubtest.Transport) {
	t.Helper()
	// Every page is sent with the login, so the later page is matched first.
	transport.ReplyFixtureWhen(t, "OwnerRepositories", "after", ownerReposPage2, fixture("owner_repositories_page_2.json"))
	transport.ReplyFixtureWhen(t, "OwnerRepositories", "login", "octo-org", fixture("owner_repositories_page_1.json"))
}

// startLoadingFilterScreen starts the app on the filter fixtures with the
// config's filter section holding filter, lets setup queue replies, and opens
// the filter screen once its first row is the viewer's.
func startLoadingFilterScreen(t *testing.T, filter string, setup func(*githubtest.Transport), opts ...harnessOption) *harness {
	t.Helper()
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", filterConfig(filter))
	transport := filterTransport(t)
	transport.Reply("Viewer", viewerReply(t))
	transport.Reply("ViewerOrganizations", organizationsReply(t))
	setup(transport)
	h := newHarness(t, transport, append([]harnessOption{withConfigDir(dir)}, opts...)...)
	h.waitForScreen("the first refresh", reviewCount.MatchString)
	h.waitForText(newerRef)
	h.openFilter()
	h.waitForFirstRow("user-a")
	return h
}

func TestLoadingAnOwnersRepositoriesPagesThroughAndListsThemByName(t *testing.T) {
	h := startLoadingFilterScreen(t, "", func(tr *githubtest.Transport) { replyOwnerRepositories(t, tr) })
	h.selectRow("octo-org")

	h.press("a")
	screen := h.waitForRow("x", "zeta-repo", 0)
	assertOrder(t, screen, "[x] user-a", "[x] octo-org", "[x] "+longRepo, "[x] alpha-repo", "[x] beta-repo",
		"[x] delta-repo", "[x] gamma-repo", "[x] kappa-repo", "[x] lambda-repo", "[x] omega-repo",
		"[x] "+sampleRepo, "[x] "+toolsRepo, "[x] zeta-repo", "[x] sample-org")
	for _, row := range []string{sampleRepo, toolsRepo} {
		if got := strings.Count(screen, "] "+row+" "); got != 1 {
			t.Errorf("a loaded repository with pull requests should be one row, got %d of %s:\n%s", got, row, screen)
		}
	}
	h.waitForRow("x", sampleRepo, 2)
	h.waitForRow("x", "octo-org", 4)

	reqs := requestsFor(h, "OwnerRepositories")
	if len(reqs) != 2 {
		t.Fatalf("sent %d repositories queries, want 2", len(reqs))
	}
	for _, r := range reqs {
		if r.Variables["login"] != "octo-org" {
			t.Errorf("repositories query asked for %v, want octo-org", r.Variables["login"])
		}
	}
	if reqs[0].Variables["after"] != nil || reqs[1].Variables["after"] != ownerReposPage2 {
		t.Errorf("repositories pages were asked for after %v and %v, want nothing then %s",
			reqs[0].Variables["after"], reqs[1].Variables["after"], ownerReposPage2)
	}
	if !strings.Contains(reqs[0].Query, "repositoryOwner(login: $login)") {
		t.Errorf("repositories query doesn't ask the repository owner, which covers users and organizations:\n%s", reqs[0].Query)
	}

	h.selectRow("zeta-repo")
	h.press(" ")
	h.waitForRow(" ", "zeta-repo", 0)
	h.waitForRow("~", "octo-org", 4)
	h.waitForFilter(nil, map[string]string{"octo-org/zeta-repo": "excluded"})
}

func TestLoadingAUserOwnersRepositories(t *testing.T) {
	h := startLoadingFilterScreen(t, "", func(tr *githubtest.Transport) {
		tr.ReplyWhen("OwnerRepositories", "login", "user-a", githubtest.Response{Body: []byte(`{"data":{"repositoryOwner":{"repositories":{
			"pageInfo":{"hasNextPage":false,"endCursor":"Y3Vyc29yOjI="},
			"nodes":[{"nameWithOwner":"user-a/other-repo"},{"nameWithOwner":"user-a/dotfiles"}]}}}}`)})
	})

	h.press("a")
	screen := h.waitForRow("x", "dotfiles", 0)
	assertOrder(t, screen, "[x] user-a", "[x] dotfiles", "[x] other-repo", "[x] octo-org")
	reqs := requestsFor(h, "OwnerRepositories")
	if len(reqs) != 1 || reqs[0].Variables["login"] != "user-a" {
		t.Fatalf("repositories queries = %+v, want one for user-a", reqs)
	}

	h.selectRow("dotfiles")
	h.press(" ")
	h.waitForRow(" ", "dotfiles", 0)
	h.waitForFilter(nil, map[string]string{"user-a/dotfiles": "excluded"})
}

func TestLoadedRepositoriesFollowTheOwnerDefaultUnlessTheyHaveAChoice(t *testing.T) {
	h := startLoadingFilterScreen(t, excludeOctoOrg+"  repositories:\n    Octo-Org/Zeta-Repo: included\n",
		func(tr *githubtest.Transport) { replyOwnerRepositories(t, tr) })
	h.waitForRow("~", "octo-org", 4)
	h.selectRow("octo-org")

	h.press("a")
	screen := h.waitForRow(" ", "omega-repo", 0)
	assertOrder(t, screen, "[~] octo-org", "[ ] alpha-repo", "[ ] omega-repo", "[ ] "+sampleRepo)
	if got := strings.Count(strings.ToLower(screen), "] zeta-repo "); got != 1 {
		t.Errorf("a loaded repository with a stored choice in another case should be one row, got %d:\n%s", got, screen)
	}
	if !ownerRow("x", "zeta-repo", 0).MatchString(screen) && !ownerRow("x", "Zeta-Repo", 0).MatchString(screen) {
		t.Errorf("zeta-repo is included by its stored choice and should be checked:\n%s", screen)
	}
}

func TestFailedRepositoriesQueryKeepsTheRowsUsable(t *testing.T) {
	release := make(chan struct{})
	h := startLoadingFilterScreen(t, "", func(tr *githubtest.Transport) {
		tr.Reply("OwnerRepositories", githubtest.Response{Err: errors.New("connection refused"), Release: release})
	})
	h.selectRow("octo-org")
	h.press("enter")
	h.waitForRow("x", sampleRepo, 2)

	h.press("a")
	h.waitForText("Loading octo-org's repositories")
	close(release)
	screen := h.waitForText("connection refused")
	if onScreen(screen, "Loading octo-org's repositories") {
		t.Errorf("a finished load should no longer say it is loading:\n%s", screen)
	}
	assertOrder(t, screen, "[x] user-a", "[x] octo-org", "[x] "+longRepo, "[x] "+sampleRepo, "[x] "+toolsRepo)

	h.press(" ")
	h.waitForRow(" ", "octo-org", 4)
	h.waitForExcludedOwners("octo-org")
}

func TestFilterScreenScrollsToKeepTheSelectedRowInView(t *testing.T) {
	h := startLoadingFilterScreen(t, "", func(tr *githubtest.Transport) { replyOwnerRepositories(t, tr) },
		withTermSize(defaultTermWidth, 14))
	h.selectRow("octo-org")
	h.press("a")
	h.waitForRow("x", "alpha-repo", 0)

	h.selectRow("zeta-repo")
	screen := h.screen.plain()
	if rowShown(screen, "user-a") || rowShown(screen, "alpha-repo") {
		t.Errorf("the list should have scrolled past its first rows:\n%s", screen)
	}

	h.selectRow("test-org")
	for range 20 {
		h.press("k")
	}
	screen = h.waitForScreen("the list to scroll back to the top", func(s string) bool {
		m := selectedRow.FindStringSubmatch(s)
		return m != nil && m[1] == "user-a"
	})
	if rowShown(screen, "zeta-repo") {
		t.Errorf("the list scrolled back to the top should not show its last rows:\n%s", screen)
	}
}

func TestLoadingAnOwnerAgainReplacesItsLoadedRepositories(t *testing.T) {
	page := func(repos string) githubtest.Response {
		return githubtest.Response{Body: []byte(`{"data":{"repositoryOwner":{"repositories":{
			"pageInfo":{"hasNextPage":false,"endCursor":"Y3Vyc29yOjI="},"nodes":[` + repos + `]}}}}`)}
	}
	h := startLoadingFilterScreen(t, "", func(tr *githubtest.Transport) {
		tr.Reply("OwnerRepositories", page(`{"nameWithOwner":"user-a/dotfiles"},{"nameWithOwner":"user-a/notes"}`))
		tr.Reply("OwnerRepositories", page(`{"nameWithOwner":"user-a/notes"}`))
	})

	h.press("a")
	h.waitForRow("x", "dotfiles", 0)
	h.press("a")
	screen := h.waitForScreen("dotfiles to be gone after the second load", func(s string) bool { return !rowShown(s, "dotfiles") })
	if got := strings.Count(screen, "] notes "); got != 1 {
		t.Errorf("a repository loaded twice should be one row, got %d:\n%s", got, screen)
	}
}
