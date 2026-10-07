package ui_test

import (
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// Cards of search_grouping.json by reference, most recently updated first.
// g4's owner is spelled differently from g1's, but names the same owner.
const (
	g1Ref = "octo-org/sample-repo#1"
	g2Ref = "user-a/sample-repo#2"
	g3Ref = "octo-org/other-repo#3"
	g4Ref = "Octo-Org/sample-repo#4"
	g5Ref = "user-a/other-repo#5"
	// g3MovedRef is g3 once search_grouping_moved.json has it transferred.
	g3MovedRef = "user-a/moved-repo#3"
)

// groupingTermHeight fits every card of search_grouping.json in one lane,
// with group headers.
const groupingTermHeight = 50

// boardQuery is the search behind the board.
const boardQuery = "is:pr is:open author:@me archived:false"

// groupingConfig is the default tags with the extra lines appended, such as
// a grouping.
func groupingConfig(extra string) string {
	return filterConfig("") + extra
}

// groupingTransport answers the board's searches with the fixtures in turn,
// the last one repeating.
func groupingTransport(t *testing.T, fixtures ...string) *githubtest.Transport {
	t.Helper()
	transport := githubtest.New()
	for _, f := range fixtures {
		transport.ReplyFixtureWhen(t, "SearchPullRequests", "query", boardQuery, fixture(f))
	}
	return transport
}

// startGrouped starts the app with config, on search_grouping.json unless
// other fixtures are named, and waits for the first refresh.
func startGrouped(t *testing.T, config string, fixtures ...string) *harness {
	t.Helper()
	if len(fixtures) == 0 {
		fixtures = []string{"search_grouping.json"}
	}
	h := newHarness(t, groupingTransport(t, fixtures...), withConfig(config), withTermSize(defaultTermWidth, groupingTermHeight))
	h.waitForText("updated 0s ago")
	return h
}

// rowsOf returns the screen row of each text, failing if one isn't on the
// screen.
func rowsOf(t *testing.T, screen string, texts ...string) []int {
	t.Helper()
	rows := make([]int, len(texts))
	for i, text := range texts {
		_, y, ok := locate(screen, text)
		if !ok {
			t.Fatalf("%q is not on screen:\n%s", text, screen)
		}
		rows[i] = y
	}
	return rows
}

// assertTopToBottom checks the texts are on screen in the order given, top
// to bottom.
func assertTopToBottom(t *testing.T, screen string, texts ...string) {
	t.Helper()
	rows := rowsOf(t, screen, texts...)
	for i := 1; i < len(rows); i++ {
		if rows[i] <= rows[i-1] {
			t.Errorf("%q (row %d) should be below %q (row %d):\n%s", texts[i], rows[i], texts[i-1], rows[i-1], screen)
		}
	}
}

// assertLaneTopToBottom checks the texts are in lane's column in the order
// given, top to bottom, each found below the one before it.
func assertLaneTopToBottom(t *testing.T, screen string, lane int, texts ...string) {
	t.Helper()
	lines := strings.Split(screen, "\n")
	row := 0
	for _, text := range texts {
		found := false
		for ; row < len(lines) && !found; row++ {
			line := []rune(lines[row])
			from, to := min(lane*(maxLaneWidth+1), len(line)), min((lane+1)*(maxLaneWidth+1), len(line))
			found = onScreen(string(line[from:to]), text)
		}
		if !found {
			t.Fatalf("lane %d doesn't show %q, in order, among %q:\n%s", lane, text, texts, screen)
		}
	}
}

// laneOf is the index of the lane drawn at the screen column of text.
func laneOf(t *testing.T, screen, text string) int {
	t.Helper()
	x, _, ok := locate(screen, text)
	if !ok {
		t.Fatalf("%q is not on screen:\n%s", text, screen)
	}
	return x / (maxLaneWidth + 1)
}

// maxLaneWidth is the width of a lane on the default terminal.
const maxLaneWidth = 36

// selected reports whether the card for ref is drawn with the selection's
// heavy border.
func selected(screen, ref string) bool {
	x, y, ok := locateCard(screen, ref)
	lines := strings.Split(screen, "\n")
	if !ok || y == 0 {
		return false
	}
	line := []rune(lines[y-1])
	return x >= 2 && x-2 < len(line) && line[x-2] == '┏'
}

func (h *harness) waitForSelected(ref string) string {
	h.t.Helper()
	return h.waitForScreen(ref+" selected", func(s string) bool { return selected(s, ref) })
}

func TestOwnerGroupingClustersCardsNewestGroupFirst(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: owner\n"))
	screen := h.waitForText("octo-org (3)")

	assertTopToBottom(t, screen, "octo-org (3)", g1Ref, g3Ref, g4Ref, "user-a (2)", g2Ref, g5Ref)
	for _, header := range []string{"octo-org (3)", "user-a (2)"} {
		if lane := laneOf(t, screen, header); lane != 0 {
			t.Errorf("%q is in lane %d, want Untagged:\n%s", header, lane, screen)
		}
	}
	if onScreen(screen, "Octo-Org (3)") {
		t.Errorf("the header should use the newest card's spelling, octo-org:\n%s", screen)
	}
	assertContains(t, headerLine(screen), "Untagged 5")
	assertContains(t, screen, "grouped by owner")
}

func TestRepositoryGroupingKeepsSameNamedReposApart(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: repository\n"))
	screen := h.waitForText("octo-org/sample-repo (2)")

	assertTopToBottom(t, screen,
		"octo-org/sample-repo (2)", g1Ref, g4Ref,
		"user-a/sample-repo (1)", g2Ref,
		"octo-org/other-repo (1)", g3Ref,
		"user-a/other-repo (1)", g5Ref,
	)
	assertContains(t, headerLine(screen), "Untagged 5")
	assertContains(t, screen, "grouped by repository")
}

func TestNoGroupingLooksAsBeforeAndLeavesTheConfigAlone(t *testing.T) {
	for name, extra := range map[string]string{
		"no grouping key": "",
		"none":            "grouping: none\n",
		"unknown value":   "grouping: sideways\n",
	} {
		t.Run(name, func(t *testing.T) {
			config := groupingConfig(extra)
			h := startGrouped(t, config)
			screen := h.waitForText(g5Ref)

			assertTopToBottom(t, screen, g1Ref, g2Ref, g3Ref, g4Ref, g5Ref)
			for _, text := range []string{"(1)", "(2)", "(3)", "grouped by"} {
				if strings.Contains(screen, text) {
					t.Errorf("ungrouped board shows %q:\n%s", text, screen)
				}
			}
			assertContains(t, headerLine(screen), "Untagged 5")
			if got := readFile(t, h.configDir, "config.yaml"); got != config {
				t.Errorf("config was rewritten:\n%s\nwant:\n%s", got, config)
			}
		})
	}
}

func TestCardKeysSkipGroupHeaders(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: owner\n"))
	h.waitForSelected(g1Ref)

	for _, ref := range []string{g3Ref, g4Ref, g2Ref, g5Ref} {
		h.press("j")
		h.waitForSelected(ref)
	}
	for _, ref := range []string{g2Ref, g4Ref, g3Ref, g1Ref} {
		h.press("k")
		h.waitForSelected(ref)
	}
}

func TestClickingAGroupHeaderSelectsAndOpensNothing(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: owner\n"))
	screen := h.waitForSelected(g1Ref)
	before := len(h.requests())

	x, y, _ := locate(screen, "user-a (2)")
	h.click(x, y)
	h.click(x, y)
	h.press("j")
	screen = h.waitForSelected(g3Ref)

	if got := len(h.requests()); got != before {
		t.Errorf("clicking a header sent %d requests", got-before)
	}
	for _, ref := range []string{g2Ref, g5Ref} {
		if selected(screen, ref) {
			t.Errorf("clicking the header selected %s:\n%s", ref, screen)
		}
	}
}

func TestMovedCardLandsInItsGroupInTheNewLane(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: owner\n"))
	h.waitForSelected(g1Ref)

	h.press("j")
	h.waitForSelected(g3Ref)
	h.press("L")
	screen := h.waitForText("octo-org (1)")
	if lane := laneOf(t, screen, "octo-org (1)"); lane != 1 {
		t.Errorf("octo-org (1) is in lane %d, want In Progress:\n%s", lane, screen)
	}
	assertTopToBottom(t, screen, "octo-org (1)", g3Ref)

	h.press("h")
	h.press("j")
	h.waitForSelected(g2Ref)
	h.press("L")
	screen = h.waitForText("user-a (1)")

	assertContains(t, headerLine(screen), "Untagged 3", "In Progress 2")
	assertLaneTopToBottom(t, screen, 0, "octo-org (2)", "SUP-1234 fix login", "Rename the config loader", "user-a (1)", "Document the release steps")
	assertLaneTopToBottom(t, screen, 1, "user-a (1)", "Add a health check", "octo-org (1)", "Tidy the build script")
	h.waitFor("the move to be saved", func() bool {
		return strings.Contains(readFile(t, h.configDir, "state.json"), `"PR_node_g2": "in-progress"`)
	})
}

func TestRefreshKeepsTheSelectionAndRegroupsAMovedRepository(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: owner\n"), "search_grouping.json", "search_grouping_moved.json")
	h.waitForSelected(g1Ref)
	h.press("j")
	h.waitForSelected(g3Ref)

	h.press("r")
	screen := h.waitForSelected(g3MovedRef)

	assertTopToBottom(t, screen, "octo-org (2)", g1Ref, g4Ref, "user-a (3)", g2Ref, g3MovedRef, g5Ref)
}

func TestSelectionStaysWhenTheGroupingChanges(t *testing.T) {
	h := startGrouped(t, groupingConfig(""))
	h.waitForSelected(g1Ref)
	h.press("j")
	h.press("j")
	h.waitForSelected(g3Ref)

	// A hand edit is picked up when the config is next read back, here after
	// the tag manager marks In Progress terminal.
	seedFile(t, h.configDir, "config.yaml", groupingConfig("grouping: repository\n"))
	h.press("t")
	h.waitForText("terminal")
	h.press("j")
	h.press("t")
	h.press("esc")
	screen := h.waitForText("grouped by repository")
	screen = h.waitForSelected(g3Ref)

	assertTopToBottom(t, screen, "octo-org/other-repo (1)", g3Ref)
	assertContains(t, readFile(t, h.configDir, "config.yaml"), "grouping: repository", "version: 1")
}

func TestFilteredPullRequestsAreLeftOutOfGroups(t *testing.T) {
	h := startGrouped(t, filterConfig("  excluded_owners: [user-a]\n")+"grouping: owner\n")
	screen := h.waitForText("octo-org (3)")

	if strings.Contains(screen, "user-a") {
		t.Errorf("an excluded owner's group or cards are shown:\n%s", screen)
	}
	assertContains(t, headerLine(screen), "Untagged 3")
}

func TestFinishedPullRequestsStayInTheirGroupBadged(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "state.json", `{"version":1,"assignments":{"PR_node_merged":"done","PR_node_closed":"done"}}`)
	transport := groupingTransport(t, "search_grouping.json")
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	h := newHarness(t, transport, withConfigDir(dir), withConfig(groupingConfig("grouping: owner\n")), withTermSize(defaultTermWidth, groupingTermHeight))
	screen := h.waitForText("octo-org/sample-repo#20")

	assertContains(t, headerLine(screen), "Done 2")
	assertTopToBottom(t, screen, "octo-org (1)", "octo-org/sample-repo#20", "user-a (1)", "user-a/other-repo#21")
	if lane := laneOf(t, screen, "octo-org/sample-repo#20"); lane != 5 {
		t.Errorf("the merged PR is in lane %d, want Done:\n%s", lane, screen)
	}
	assertContains(t, cardLine(t, screen, "octo-org/sample-repo#20", 3), "Merged")
	assertContains(t, cardLine(t, screen, "user-a/other-repo#21", 3), "Closed")
}

func TestLongGroupedLaneScrollsWithTheSelection(t *testing.T) {
	h := newHarness(t, groupingTransport(t, "search_grouping.json"), withConfig(groupingConfig("grouping: owner\n")), withTermSize(defaultTermWidth, 20))
	h.waitForSelected(g1Ref)

	for range 4 {
		h.press("j")
	}
	screen := h.waitForSelected(g2Ref)
	assertContains(t, screen, "user-a (2)")
	if onScreen(screen, g1Ref) {
		t.Errorf("the lane should have scrolled past %s:\n%s", g1Ref, screen)
	}

	for range 4 {
		h.press("k")
	}
	screen = h.waitForSelected(g1Ref)
	assertContains(t, screen, "octo-org (3)")
}

func TestGroupingSendsTheSameRequests(t *testing.T) {
	queries := map[string]string{}
	for _, extra := range []string{"", "grouping: owner\n"} {
		h := startGrouped(t, groupingConfig(extra))
		reqs := h.waitForRequests(1)
		queries[extra] = reqs[0].Query
		if len(reqs) != 1 {
			t.Errorf("with %q, %d requests besides review requests, want 1", extra, len(reqs))
		}
	}
	if queries[""] != queries["grouping: owner\n"] {
		t.Errorf("grouping changed the board's query")
	}
}

func TestGroupHeaderIsARuleAsWideAsTheCards(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: owner\n"))
	screen := h.waitForText("octo-org (3)")

	_, row, _ := locate(screen, "octo-org (3)")
	line := []rune(strings.Split(screen, "\n")[row])
	header := strings.TrimRight(string(line[:min(maxLaneWidth, len(line))]), " ")
	if want := "── octo-org " + strings.Repeat("─", maxLaneWidth-len("── octo-org ")-2) + " 3"; header != want {
		t.Errorf("header is %q, want %q:\n%s", header, want, screen)
	}
}
