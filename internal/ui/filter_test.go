package ui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// Cards of search_board.json, search_review_requests.json and
// nodes_tracked.json by reference.
const (
	newerRef        = "user-a/other-repo#42"
	olderRef        = "octo-org/sample-repo#7"
	longRepoRef     = "octo-org/a-very-long-sample-repository-name#1234"
	lockedReviewTxt = "Speed up the lint step"
	openReviewTxt   = "Add pagination to the export API"
	mergedTitle     = "Ship the upload worker"
	closedTitle     = "Try a different cache"
)

// filterConfig is the default tags with filter appended as the config's
// filter section.
func filterConfig(filter string) string {
	cfg := `version: 1
tags:
  - {id: in-progress, name: In Progress, color: "#58A6FF"}
  - {id: review, name: Review, color: "#D29922"}
  - {id: testing, name: Testing, color: "#A371F7"}
  - {id: demo, name: Demo, color: "#3FB950"}
  - {id: done, name: Done, color: "#8B949E", terminal: true}
`
	if filter != "" {
		cfg += "filter:\n" + filter
	}
	return cfg
}

// reviewCount is the review requests tab's label once a refresh has landed.
var reviewCount = regexp.MustCompile(`Review requests [0-9]`)

const excludeOctoOrg = "  excluded_owners: [octo-org]\n"

// filterTransport answers the board search with the board fixture, the review
// requests search with both review requests, and every fetch by ID with the
// merged and the closed pull request.
func filterTransport(t *testing.T) *githubtest.Transport {
	t.Helper()
	transport := boardTransport(t)
	replyReviewRequests(t, transport, "search_review_requests.json")
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	return transport
}

// startFiltered starts the app on dir with the config file holding filter,
// and waits until the first refresh is on screen.
func startFiltered(t *testing.T, dir, filter string) *harness {
	t.Helper()
	seedFile(t, dir, "config.yaml", filterConfig(filter))
	h := newHarness(t, filterTransport(t), withConfigDir(dir))
	h.waitForScreen("the first refresh", reviewCount.MatchString)
	return h
}

// restart quits the app so the next harness on its config directory starts
// as a restart would.
func (h *harness) restart() {
	h.t.Helper()
	h.press("q")
	h.waitFinished()
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// The tagged older PR (Testing), the merged PR archived from Testing and the
// closed PR archived untagged.
const filterState = `{"version":1,"assignments":{"PR_node_older":"testing"},"archived":[{"id":"PR_node_merged","open":false,"tag":"testing"},{"id":"PR_node_closed","open":false}]}`

// assertFilterState checks the state file still holds filterState's
// assignments and archived list.
func assertFilterState(t *testing.T, dir, after string) {
	t.Helper()
	got, ok := readState(dir)
	want := stateJSON{
		Version:     1,
		Assignments: map[string]string{olderPR: "testing"},
		Archived:    []archivedJSON{{ID: mergedPR, Tag: "testing"}, {ID: closedPR}},
	}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("after %s, state = %+v, want %+v", after, got, want)
	}
}

func TestExcludedOwnerIsHiddenOnEveryTabAndComesBackUnchanged(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "state.json", filterState)
	h := startFiltered(t, dir, excludeOctoOrg)

	screen := h.waitForText(newerRef)
	assertContains(t, headerLine(screen), "Untagged 1", "Testing 0")
	for _, ref := range []string{olderRef, longRepoRef} {
		if onScreen(screen, ref) {
			t.Errorf("%s is in an excluded owner and should be hidden:\n%s", ref, screen)
		}
	}
	assertContains(t, screen, "Review requests 0")

	h.press("tab")
	screen = h.waitForText("Review requests 0")
	for _, title := range []string{openReviewTxt, lockedReviewTxt} {
		if strings.Contains(screen, title) {
			t.Errorf("review request %q is in an excluded owner and should be hidden:\n%s", title, screen)
		}
	}

	h.press("tab")
	screen = h.waitForText(closedTitle)
	assertContains(t, screen, "Archived 1")
	if strings.Contains(screen, mergedTitle) {
		t.Errorf("the archived PR in an excluded owner should be hidden:\n%s", screen)
	}
	h.restart()
	assertFilterState(t, dir, "excluding")

	h = startFiltered(t, dir, "")
	screen = h.waitForText(olderRef)
	assertContains(t, headerLine(screen), "Untagged 2", "Testing 1")
	assertContains(t, screen, "Review requests 2")
	h.clickText("Archived 2")
	screen = h.waitForText(mergedTitle)
	assertContains(t, screen, closedTitle)
	h.restart()
	assertFilterState(t, dir, "re-including")
}

func TestExcludedOwnerHidesARepositorySeenOnALaterRefresh(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board_older_only.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport, withConfig(filterConfig(excludeOctoOrg)))
	h.waitForText("Untagged 0")

	h.refreshUntil(2)
	screen := h.waitForText(newerRef)
	assertContains(t, headerLine(screen), "Untagged 1")
	if onScreen(screen, longRepoRef) {
		t.Errorf("a repository first seen on a later refresh should follow its excluded owner:\n%s", screen)
	}
}

func TestRepositoryChoiceOverridesItsOwner(t *testing.T) {
	h := startFiltered(t, t.TempDir(), excludeOctoOrg+`  repositories:
    octo-org/sample-repo: included
    user-a/other-repo: excluded
`)

	screen := h.waitForText(olderRef)
	assertContains(t, headerLine(screen), "Untagged 1")
	for _, ref := range []string{newerRef, longRepoRef} {
		if onScreen(screen, ref) {
			t.Errorf("%s should be hidden:\n%s", ref, screen)
		}
	}
	h.press("tab")
	screen = h.waitForText(openReviewTxt)
	assertContains(t, screen, "Review requests 1")
	if strings.Contains(screen, lockedReviewTxt) {
		t.Errorf("the review request in another repository of the excluded owner should be hidden:\n%s", screen)
	}
}

func TestFilterNamesMatchWhateverTheirCapitalisation(t *testing.T) {
	h := startFiltered(t, t.TempDir(), `  excluded_owners: [OCTO-ORG]
  repositories:
    Octo-Org/Sample-Repo: included
    USER-A/Other-Repo: excluded
`)

	screen := h.waitForText(olderRef)
	assertContains(t, headerLine(screen), "Untagged 1")
	if onScreen(screen, newerRef) || onScreen(screen, longRepoRef) {
		t.Errorf("the filter should apply whatever the capitalisation:\n%s", screen)
	}
}

func TestConfigWithoutAFilterShowsEverythingAndIsNotRewritten(t *testing.T) {
	dir := t.TempDir()
	h := startFiltered(t, dir, "")

	screen := h.waitForText(olderRef)
	assertContains(t, headerLine(screen), "Untagged 3")
	h.settle(2 * time.Second)
	h.restart()
	if got := readFile(t, dir, "config.yaml"); got != filterConfig("") {
		t.Errorf("the config was rewritten:\n%s", got)
	}
}

func TestSavingTagsKeepsTheFilter(t *testing.T) {
	h := startFiltered(t, t.TempDir(), excludeOctoOrg+"  repositories:\n    octo-org/sample-repo: included\n")
	h.waitForText(olderRef)

	h.openTagManager()
	h.press("n")
	h.press("Extra")
	h.press("enter")
	h.waitForText("Color for Extra")
	h.press("enter")
	h.waitForConfig("with the new tag", func(c configYAML) bool { return len(c.Tags) == 6 })

	var cfg struct {
		Version int `yaml:"version"`
		Filter  struct {
			ExcludedOwners []string          `yaml:"excluded_owners"`
			Repositories   map[string]string `yaml:"repositories"`
		} `yaml:"filter"`
	}
	readYAML(t, h.configDir, &cfg)
	if cfg.Version != 1 {
		t.Errorf("config version = %d, want 1", cfg.Version)
	}
	if want := []string{"octo-org"}; !reflect.DeepEqual(cfg.Filter.ExcludedOwners, want) {
		t.Errorf("excluded owners = %v, want %v", cfg.Filter.ExcludedOwners, want)
	}
	if want := map[string]string{"octo-org/sample-repo": "included"}; !reflect.DeepEqual(cfg.Filter.Repositories, want) {
		t.Errorf("repositories = %v, want %v", cfg.Filter.Repositories, want)
	}
}

func TestSavingTagsAddsNoFilter(t *testing.T) {
	h := startFiltered(t, t.TempDir(), "")
	h.waitForText(olderRef)

	h.openTagManager()
	h.press("n")
	h.press("Extra")
	h.press("enter")
	h.waitForText("Color for Extra")
	h.press("enter")
	h.waitForConfig("with the new tag", func(c configYAML) bool { return len(c.Tags) == 6 })
	if got := readFile(t, h.configDir, "config.yaml"); strings.Contains(got, "filter") {
		t.Errorf("saving tags wrote a filter nobody set:\n%s", got)
	}
}

func TestPRMergedWhileHiddenComesBackMerged(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "state.json", `{"version":1,"assignments":{"PR_node_merged":"demo"}}`)
	h := startFiltered(t, dir, excludeOctoOrg)
	screen := h.waitForText(newerRef)
	h.waitFor("the tagged PR fetched by ID", func() bool { return len(byIDRequests(h)) == 1 })
	screen = h.waitForText("Demo 0")
	if strings.Contains(screen, mergedTitle) {
		t.Errorf("the merged PR is in an excluded owner and should be hidden:\n%s", screen)
	}
	h.restart()

	h = startFiltered(t, dir, "")
	screen = h.waitForText(mergedTitle)
	assertContains(t, headerLine(screen), "Demo 1")
}

func TestHiddenPRsGetNoSnapshots(t *testing.T) {
	h := newHarness(t, statusTransport(t), withConfig(filterConfig(excludeOctoOrg)))
	h.waitForText("user-a/other-repo#13")

	snaps := h.waitForSnapshots("PR_node_behind", "PR_node_paged")
	for _, id := range []string{"PR_node_conflict", "PR_node_checking"} {
		if _, ok := snaps[id]; ok {
			t.Errorf("hidden %s was snapshotted", id)
		}
	}
}

func TestActivityWhileHiddenIsMarkedWhenShownAgain(t *testing.T) {
	dir := baselineStatusBoard(t)
	editSnapshot(t, dir, "PR_node_conflict", "comments", 0)
	before := readFile(t, dir, "state.json")

	seedFile(t, dir, "config.yaml", filterConfig(excludeOctoOrg))
	h := newHarness(t, statusTransport(t), withConfigDir(dir))
	screen := h.waitForText("user-a/other-repo#13")
	if strings.Contains(screen, "changed") {
		t.Errorf("a hidden PR's activity should not be reported:\n%s", screen)
	}
	h.settle(2 * time.Second)
	h.restart()
	if got := readFile(t, dir, "state.json"); got != before {
		t.Errorf("the hidden PR's snapshot should stay frozen; state is now:\n%s", got)
	}

	seedFile(t, dir, "config.yaml", filterConfig(""))
	h = newHarness(t, statusTransport(t), withConfigDir(dir))
	screen = h.waitForText("1 PR changed")
	assertMarkers(t, screen, statusRefs, map[string]string{"PR_node_conflict": "dot"})
}

func TestSearchesAreTheSameWithAndWithoutAFilter(t *testing.T) {
	sent := func(filter string) []githubtest.Request {
		h := startFiltered(t, t.TempDir(), filter)
		var searches []githubtest.Request
		for _, r := range h.transport.Requests() {
			if r.Operation == "SearchPullRequests" {
				r.Header = nil
				searches = append(searches, r)
			}
		}
		h.restart()
		// The two searches run side by side, so they arrive in either order.
		slices.SortFunc(searches, func(a, b githubtest.Request) int {
			return strings.Compare(fmt.Sprint(a.Variables["query"]), fmt.Sprint(b.Variables["query"]))
		})
		return searches
	}

	plain := sent("")
	filtered := sent(excludeOctoOrg + "  repositories:\n    user-a/other-repo: excluded\n    octo-org/sample-repo: included\n")
	if len(plain) != 2 || !reflect.DeepEqual(plain, filtered) {
		t.Errorf("searches differ with a filter:\nwithout: %+v\nwith:    %+v", plain, filtered)
	}
}

func readYAML(t *testing.T, dir string, out any) {
	t.Helper()
	if err := yaml.Unmarshal([]byte(readFile(t, dir, "config.yaml")), out); err != nil {
		t.Fatalf("config is not YAML: %v", err)
	}
}
