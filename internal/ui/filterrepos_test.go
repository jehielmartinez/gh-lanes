package ui_test

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Repository rows of octo-org in the filter fixtures, by name.
const (
	sampleRepo = "sample-repo"
	longRepo   = "a-very-long-sample-repository-name"
	toolsRepo  = "tools-repo"
)

// selectedRow is the filter screen row the cursor is on.
var selectedRow = regexp.MustCompile(`› +\[[x ~]\] (\S+)`)

// selectRow moves the filter screen's cursor down until it is on name's row.
func (h *harness) selectRow(name string) {
	h.t.Helper()
	selected := func(s string) string {
		if m := selectedRow.FindStringSubmatch(s); m != nil {
			return m[1]
		}
		return ""
	}
	for range 20 {
		before := selected(h.screen.plain())
		if before == name {
			return
		}
		h.press("j")
		h.waitForScreen("the cursor to move towards "+name, func(s string) bool { return selected(s) != before })
	}
	h.t.Fatalf("no filter screen row %s:\n%s", name, h.screen.plain())
}

// rowShown reports whether the filter screen lists name, whatever its check.
func rowShown(screen, name string) bool {
	return regexp.MustCompile(`\[[x ~]\] ` + regexp.QuoteMeta(name) + `\s`).MatchString(screen)
}

// waitForFilter waits until the config file's filter is exactly owners and
// repos, a missing section counting as empty.
func (h *harness) waitForFilter(owners []string, repos map[string]string) {
	h.t.Helper()
	h.waitFor("the config filter to be "+strings.Join(owners, ",")+" "+formatMap(repos), func() bool {
		var cfg filterYAML
		raw, err := os.ReadFile(filepath.Join(h.configDir, "config.yaml"))
		if err != nil || yaml.Unmarshal(raw, &cfg) != nil || cfg.Version != 1 {
			return false
		}
		var gotOwners []string
		gotRepos := map[string]string{}
		if cfg.Filter != nil {
			gotOwners = cfg.Filter.ExcludedOwners
			maps.Copy(gotRepos, cfg.Filter.Repositories)
		}
		if repos == nil {
			repos = map[string]string{}
		}
		return slices.Equal(gotOwners, owners) && reflect.DeepEqual(gotRepos, repos)
	})
}

func TestRepositoriesListUnderTheirOwnerByNameAndExpandAndCollapse(t *testing.T) {
	h := startFilterScreen(t, "", viewerReply(t))
	screen := h.waitForFirstRow("user-a")
	for _, repo := range []string{sampleRepo, longRepo, toolsRepo, "other-repo"} {
		if rowShown(screen, repo) {
			t.Errorf("owners should start collapsed, but %s is listed:\n%s", repo, screen)
		}
	}

	h.selectRow("octo-org")
	h.press("enter")
	screen = h.waitForRow("x", toolsRepo, 1)
	for _, row := range []*regexp.Regexp{ownerRow("x", longRepo, 1), ownerRow("x", sampleRepo, 2)} {
		if !row.MatchString(screen) {
			t.Errorf("filter screen is missing %s:\n%s", row, screen)
		}
	}
	assertOrder(t, screen, "[x] octo-org", "[x] "+longRepo, "[x] "+sampleRepo, "[x] "+toolsRepo)
	if rowShown(screen, "other-repo") {
		t.Errorf("user-a was not expanded, but its repository is listed:\n%s", screen)
	}

	h.press("h")
	h.waitForScreen("octo-org to collapse", func(s string) bool { return !rowShown(s, sampleRepo) })
	h.press("l")
	h.waitForRow("x", sampleRepo, 2)
	h.press("enter")
	h.waitForScreen("octo-org to collapse", func(s string) bool { return !rowShown(s, sampleRepo) })
}

func TestCollapsingFromARepositoryRowSelectsItsOwner(t *testing.T) {
	h := startFilterScreen(t, "", viewerReply(t))
	h.waitForFirstRow("user-a")
	h.selectRow("octo-org")
	h.press("l")
	h.waitForRow("x", sampleRepo, 2)
	h.selectRow(sampleRepo)

	h.press("h")
	h.waitForScreen("octo-org collapsed and selected", func(s string) bool {
		m := selectedRow.FindStringSubmatch(s)
		return !rowShown(s, sampleRepo) && m != nil && m[1] == "octo-org"
	})
}

func TestPartialOwnersStartExpanded(t *testing.T) {
	h := startFilterScreen(t, excludeOctoOrg+"  repositories:\n    octo-org/sample-repo: included\n", viewerReply(t))

	h.waitForFirstRow("user-a")
	screen := h.waitForRow("~", "octo-org", 4)
	assertOrder(t, screen, "[~] octo-org", "[ ] "+longRepo, "[x] "+sampleRepo, "[ ] "+toolsRepo)
	if rowShown(screen, "other-repo") {
		t.Errorf("user-a is not partial and should start collapsed:\n%s", screen)
	}
}

func TestRecheckingARepositoryUnderAnExcludedOwnerShowsItsCards(t *testing.T) {
	h := startFilterScreen(t, excludeOctoOrg, viewerReply(t))
	h.waitForFirstRow("user-a")
	h.selectRow("octo-org")
	h.press("enter")
	h.waitForRow(" ", toolsRepo, 1)
	h.selectRow(sampleRepo)

	h.press(" ")
	h.waitForRow("x", sampleRepo, 2)
	h.waitForRow("~", "octo-org", 4)
	h.waitForFilter([]string{"octo-org"}, map[string]string{"octo-org/sample-repo": "included"})
	screen := h.waitForScreen("the board behind to show sample-repo", func(s string) bool {
		return strings.Contains(headerLine(s), "Untagged 2")
	})
	assertContains(t, screen, "Review requests 1")

	h.press("esc")
	screen = h.waitForText(olderRef)
	if onScreen(screen, longRepoRef) {
		t.Errorf("only the re-checked repository should come back:\n%s", screen)
	}
}

func TestUncheckingARepositoryHidesOnlyItsCardsAndRecheckingDropsTheChoice(t *testing.T) {
	h := startFilterScreen(t, "", viewerReply(t))
	h.waitForFirstRow("user-a")
	h.selectRow("octo-org")
	h.press("enter")
	h.waitForRow("x", toolsRepo, 1)
	h.selectRow(sampleRepo)

	h.press(" ")
	h.waitForRow(" ", sampleRepo, 2)
	h.waitForRow("~", "octo-org", 4)
	h.waitForFilter(nil, map[string]string{"octo-org/sample-repo": "excluded"})
	screen := h.waitForScreen("the board behind to drop sample-repo", func(s string) bool {
		return strings.Contains(headerLine(s), "Untagged 2")
	})
	assertContains(t, screen, "Review requests 1")
	if onScreen(screen, olderRef) || !onScreen(screen, newerRef) {
		t.Errorf("only sample-repo's cards should be hidden:\n%s", screen)
	}

	h.press(" ")
	h.waitForRow("x", sampleRepo, 2)
	h.waitForRow("x", "octo-org", 4)
	h.waitForFilter(nil, nil)
	if got := readFile(t, h.configDir, "config.yaml"); strings.Contains(got, "filter") {
		t.Errorf("a choice matching its owner default should be dropped:\n%s", got)
	}
}

func TestTogglingAnOwnerClearsTheChoicesUnderIt(t *testing.T) {
	h := startFilterScreen(t, `  excluded_owners: [octo-org]
  repositories:
    Octo-Org/Sample-Repo: included
    octo-org/tools-repo: excluded
    other-org/side-repo: excluded
    user-a/other-repo: included
`, viewerReply(t))
	h.waitForFirstRow("user-a")
	h.waitForRow("~", "octo-org", 4)
	h.selectRow("octo-org")

	h.press(" ")
	h.waitForRow("x", "octo-org", 4)
	screen := h.waitForRow("x", sampleRepo, 2)
	assertOrder(t, screen, "[x] octo-org", "[x] "+longRepo, "[x] "+sampleRepo, "[x] "+toolsRepo)
	h.waitForFilter(nil, map[string]string{"other-org/side-repo": "excluded"})

	h.press(" ")
	h.waitForRow(" ", "octo-org", 4)
	h.waitForRow(" ", sampleRepo, 2)
	h.waitForFilter([]string{"octo-org"}, map[string]string{"other-org/side-repo": "excluded"})
}

func TestRepositoryWithAStoredChoiceIsListedWithoutPullRequests(t *testing.T) {
	h := startFilterScreen(t, `  repositories:
    octo-org/old-repo: excluded
    other-org/side-repo: excluded
`, viewerReply(t))

	h.waitForFirstRow("user-a")
	screen := h.waitForRow(" ", "old-repo", 0)
	assertOrder(t, screen, "[~] octo-org", "[x] "+longRepo, "[ ] old-repo", "[x] "+sampleRepo)
	screen = h.waitForRow(" ", "side-repo", 0)
	assertOrder(t, screen, "[~] other-org", "[ ] side-repo")
	if !ownerRow("~", "other-org", 0).MatchString(screen) {
		t.Errorf("filter screen is missing other-org as partial:\n%s", screen)
	}
}
