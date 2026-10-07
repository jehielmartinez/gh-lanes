package ui_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

const filterHint = "space toggle"

// filterYAML is the config file's filter section.
type filterYAML struct {
	Version int `yaml:"version"`
	Filter  *struct {
		ExcludedOwners []string          `yaml:"excluded_owners"`
		Repositories   map[string]string `yaml:"repositories"`
	} `yaml:"filter"`
}

// waitForExcludedOwners waits until the config file excludes exactly want.
func (h *harness) waitForExcludedOwners(want ...string) {
	h.t.Helper()
	h.waitFor("the config to exclude "+strings.Join(want, ", "), func() bool {
		var cfg filterYAML
		raw, err := os.ReadFile(filepath.Join(h.configDir, "config.yaml"))
		if err != nil || yaml.Unmarshal(raw, &cfg) != nil || cfg.Version != 1 {
			return false
		}
		var got []string
		if cfg.Filter != nil {
			got = cfg.Filter.ExcludedOwners
		}
		return slices.Equal(got, want)
	})
}

// startFilterScreen starts the app on the filter fixtures with the config's
// filter section holding filter, the viewer query answered with reply, and
// opens the filter screen once the first refresh is on screen.
func startFilterScreen(t *testing.T, filter string, reply githubtest.Response) *harness {
	t.Helper()
	return startFilterScreenWith(t, filter, reply, organizationsReply(t))
}

// startFilterScreenWith is startFilterScreen with the organizations query
// answered with orgs.
func startFilterScreenWith(t *testing.T, filter string, viewer, orgs githubtest.Response) *harness {
	t.Helper()
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", filterConfig(filter))
	transport := filterTransport(t)
	transport.Reply("Viewer", viewer)
	transport.Reply("ViewerOrganizations", orgs)
	h := newHarness(t, transport, withConfigDir(dir))
	h.waitForScreen("the first refresh", reviewCount.MatchString)
	h.waitForText(newerRef)
	h.openFilter()
	return h
}

func viewerReply(t *testing.T) githubtest.Response {
	t.Helper()
	body, err := os.ReadFile(fixture("viewer.json"))
	if err != nil {
		t.Fatal(err)
	}
	return githubtest.Response{Body: body}
}

// organizationsReply answers the organizations query with octo-org, which
// has pull requests, and sample-org and test-org, which have none.
func organizationsReply(t *testing.T) githubtest.Response {
	t.Helper()
	body, err := os.ReadFile(fixture("viewer_organizations.json"))
	if err != nil {
		t.Fatal(err)
	}
	return githubtest.Response{Body: body}
}

// openFilter opens the filter screen over the tab in view.
func (h *harness) openFilter() string {
	h.t.Helper()
	h.press("f")
	return h.waitForText(filterHint)
}

// ownerRow matches the filter screen row of an owner, with its checkbox and
// count.
func ownerRow(check, owner string, count int) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta("["+check+"] "+owner) + `\s+` + strconv.Itoa(count) + `(\s|$)`)
}

var checkbox = regexp.MustCompile(`\[[x ~]\] (\S+)`)

// waitForFirstRow waits until the filter screen's first row is the owner's,
// which for the viewer means its account has been loaded.
func (h *harness) waitForFirstRow(owner string) string {
	h.t.Helper()
	return h.waitForScreen(owner+" as the first row", func(s string) bool {
		first := checkbox.FindStringSubmatch(s)
		return first != nil && first[1] == owner
	})
}

// waitForRow waits until the filter screen shows the owner's row.
func (h *harness) waitForRow(check, owner string, count int) string {
	h.t.Helper()
	row := ownerRow(check, owner, count)
	return h.waitForScreen(row.String(), row.MatchString)
}

func TestFilterScreenOpensFromEveryTabAndEscClosesIt(t *testing.T) {
	h := startFilterScreen(t, "", viewerReply(t))
	closed := func(s string) bool { return !strings.Contains(s, filterHint) }
	h.press("esc")
	h.waitForScreen("the filter screen to close", closed)

	h.press("tab")
	h.waitForText(openReviewTxt)
	h.openFilter()
	h.press("esc")
	h.waitForScreen("the filter screen to close", closed)

	h.press("tab")
	h.waitForText("Archived 0")
	h.openFilter()
	h.press("esc")
	h.waitForScreen("the filter screen to close", closed)
}

func TestFilterScreenListsTheViewerFirstThenOwnersByCount(t *testing.T) {
	h := startFilterScreen(t, "  excluded_owners: [old-org]\n", viewerReply(t))

	h.waitForFirstRow("user-a")
	screen := h.waitForRow("x", "user-a", 1)
	assertContains(t, screen, "Filter")
	for _, row := range []*regexp.Regexp{ownerRow("x", "octo-org", 4), ownerRow(" ", "old-org", 0)} {
		if !row.MatchString(screen) {
			t.Errorf("filter screen is missing %s:\n%s", row, screen)
		}
	}
	assertOrder(t, screen, "[x] user-a", "[x] octo-org", "[ ] old-org")
	if got := len(requestsFor(h, "Viewer")); got != 1 {
		t.Errorf("sent %d viewer queries on opening the filter screen, want 1", got)
	}
}

func TestFilterScreenListsTheViewerWithNoPullRequests(t *testing.T) {
	h := startFilterScreen(t, "", githubtest.Response{Body: []byte(`{"data":{"viewer":{"login":"user-z"}}}`)})

	screen := h.waitForRow("x", "user-z", 0)
	assertOrder(t, screen, "[x] user-z", "[x] octo-org", "[x] user-a")
}

func TestTogglingAnOwnerHidesItsCardsAndSavesOnlyTheExclusion(t *testing.T) {
	h := startFilterScreen(t, "", viewerReply(t))
	h.waitForFirstRow("user-a")

	h.press("j")
	h.press(" ")
	screen := h.waitForRow(" ", "octo-org", 4)
	h.waitForExcludedOwners("octo-org")
	screen = h.waitForScreen("the board behind to drop octo-org", func(s string) bool {
		return strings.Contains(headerLine(s), "Untagged 1")
	})
	assertContains(t, screen, "Review requests 0")

	h.press(" ")
	h.waitForRow("x", "octo-org", 4)
	h.waitForExcludedOwners()
	if got := readFile(t, h.configDir, "config.yaml"); strings.Contains(got, "octo-org") {
		t.Errorf("re-including an owner should leave no trace of it in the config:\n%s", got)
	}

	h.press("esc")
	screen = h.waitForText(olderRef)
	assertContains(t, headerLine(screen), "Untagged 3")
	assertContains(t, screen, "Review requests 2")
}

func TestStoredExclusionInAnotherCaseIsOneRowAndCanBeUndone(t *testing.T) {
	h := startFilterScreen(t, "  excluded_owners: [OCTO-ORG]\n", viewerReply(t))

	h.waitForFirstRow("user-a")
	screen := h.waitForRow(" ", "octo-org", 4)
	if got := strings.Count(strings.ToLower(screen), "] octo-org"); got != 1 {
		t.Errorf("one owner in two spellings should be one row, got %d:\n%s", got, screen)
	}

	h.press("j")
	h.press(" ")
	h.waitForRow("x", "octo-org", 4)
	h.waitForExcludedOwners()
}

func TestFailedViewerQueryKeepsTheRowsUsable(t *testing.T) {
	h := startFilterScreen(t, "", githubtest.Response{Err: errors.New("connection refused")})

	screen := h.waitForText("connection refused")
	assertOrder(t, screen, "[x] octo-org", "[x] user-a")

	h.press(" ")
	h.waitForRow(" ", "octo-org", 4)
	h.waitForExcludedOwners("octo-org")
}

var filterHelp = regexp.MustCompile(`\bf\s+filter\b`)

func TestFullHelpListsTheFilterKey(t *testing.T) {
	h := startBoard(t)
	h.press("?")
	h.waitForScreen("the filter key in the board's full help", func(s string) bool {
		return strings.Contains(s, "close help") && filterHelp.MatchString(s)
	})

	h.press("tab")
	h.waitForScreen("the filter key in the review requests' full help", func(s string) bool {
		return strings.Contains(s, "No review requests.") && filterHelp.MatchString(s)
	})
}

func TestFilterScreenListsTheViewersOrganizationsWithNoPullRequests(t *testing.T) {
	h := startFilterScreen(t, "  excluded_owners: [old-org]\n", viewerReply(t))

	h.waitForFirstRow("user-a")
	screen := h.waitForRow("x", "sample-org", 0)
	for _, row := range []*regexp.Regexp{ownerRow("x", "test-org", 0), ownerRow("x", "octo-org", 4)} {
		if !row.MatchString(screen) {
			t.Errorf("filter screen is missing %s:\n%s", row, screen)
		}
	}
	if got := strings.Count(screen, "] octo-org"); got != 1 {
		t.Errorf("an organization with pull requests should be one row, got %d:\n%s", got, screen)
	}
	assertOrder(t, screen, "[x] user-a", "[x] octo-org", "[ ] old-org", "[x] sample-org", "[x] test-org")
	reqs := requestsFor(h, "ViewerOrganizations")
	if len(reqs) != 1 {
		t.Fatalf("sent %d organizations queries on opening the filter screen, want 1", len(reqs))
	}
	if !strings.Contains(reqs[0].Query, "viewer") || !strings.Contains(reqs[0].Query, "organizations(") {
		t.Errorf("organizations query doesn't ask for the viewer's organizations:\n%s", reqs[0].Query)
	}
}

func TestExcludingAnOrganizationWithNoPullRequestsSavesIt(t *testing.T) {
	h := startFilterScreen(t, "", viewerReply(t))
	h.waitForFirstRow("user-a")
	h.waitForRow("x", "sample-org", 0)

	h.press("j")
	h.press("j")
	h.press(" ")
	h.waitForRow(" ", "sample-org", 0)
	h.waitForExcludedOwners("sample-org")
}

func TestOrganizationsArePagedThrough(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", filterConfig(""))
	transport := filterTransport(t)
	transport.Reply("Viewer", viewerReply(t))
	transport.Reply("ViewerOrganizations", githubtest.Response{Body: []byte(`{"data":{"viewer":{"organizations":{
		"pageInfo":{"hasNextPage":true,"endCursor":"page-2"},"nodes":[{"login":"sample-org"}]}}}}`)})
	transport.ReplyWhen("ViewerOrganizations", "after", "page-2", githubtest.Response{Body: []byte(`{"data":{"viewer":{"organizations":{
		"pageInfo":{"hasNextPage":false,"endCursor":"page-3"},"nodes":[{"login":"test-org"}]}}}}`)})
	h := newHarness(t, transport, withConfigDir(dir))
	h.waitForScreen("the first refresh", reviewCount.MatchString)
	h.openFilter()

	screen := h.waitForRow("x", "test-org", 0)
	assertContains(t, screen, "[x] sample-org")
	reqs := requestsFor(h, "ViewerOrganizations")
	if len(reqs) != 2 {
		t.Fatalf("sent %d organizations queries, want 2", len(reqs))
	}
	if reqs[0].Variables["after"] != nil || reqs[1].Variables["after"] != "page-2" {
		t.Errorf("organizations pages were asked for after %v and %v, want nothing then page-2",
			reqs[0].Variables["after"], reqs[1].Variables["after"])
	}
}

func TestFailedOrganizationsQueryKeepsTheRowsUsable(t *testing.T) {
	h := startFilterScreenWith(t, "", viewerReply(t), githubtest.Response{Err: errors.New("connection refused")})

	h.waitForText("connection refused")
	screen := h.waitForFirstRow("user-a")
	assertContains(t, screen, "connection refused")
	assertOrder(t, screen, "[x] user-a", "[x] octo-org")

	h.press("j")
	h.press(" ")
	h.waitForRow(" ", "octo-org", 4)
	h.waitForExcludedOwners("octo-org")
}

func TestRefreshSendsNoOrganizationsQuery(t *testing.T) {
	h := startFiltered(t, t.TempDir(), "")
	h.press("r")
	h.waitFor("a second board search", func() bool {
		n := 0
		for _, r := range requestsFor(h, "SearchPullRequests") {
			if !isReviewSearch(r) {
				n++
			}
		}
		return n >= 2
	})
	if got := len(requestsFor(h, "ViewerOrganizations")); got != 0 {
		t.Errorf("sent %d organizations queries without opening the filter screen, want 0", got)
	}
}
