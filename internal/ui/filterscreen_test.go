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
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", filterConfig(filter))
	transport := filterTransport(t)
	transport.Reply("Viewer", reply)
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
	h.waitForRow("x", "user-a", 1)

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
