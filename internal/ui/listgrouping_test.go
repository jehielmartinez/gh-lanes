package ui_test

import (
	"strings"
	"testing"
)

// Cards of search_grouping_reviews.json by reference, most recently updated
// first.
const (
	r1Ref = "octo-org/review-repo#11"
	r2Ref = "user-b/sample-repo#12"
	r3Ref = "octo-org/sample-repo#13"
	r4Ref = "user-b/tools-repo#14"
	r5Ref = "octo-org/review-repo#15"
)

// Cards of nodes_grouping_archived.json by reference, most recently updated
// first: a merged, a closed and an open pull request.
const (
	mergedArchivedRef = "octo-org/sample-repo#20"
	closedArchivedRef = "user-a/other-repo#21"
	openArchivedRef   = "octo-org/other-repo#22"
)

// groupingArchivedState archives the three pull requests of
// nodes_grouping_archived.json from the board.
const groupingArchivedState = `{"version":1,"assignments":{},"archived":[` +
	`{"id":"PR_node_merged","open":false},` +
	`{"id":"PR_node_closed","open":false},` +
	`{"id":"PR_node_archived_open","open":true}]}`

// startGroupedLists starts the app with config on search_grouping.json, the
// review requests of search_grouping_reviews.json and state, in a terminal
// height lines tall, and waits for the first refresh. The archived tab
// fetches nodes_grouping_archived.json.
func startGroupedLists(t *testing.T, config, state string, height int) *harness {
	t.Helper()
	return startGroupedListsArchiving(t, config, state, height, "nodes_grouping_archived.json")
}

// startGroupedListsArchiving is startGroupedLists with the archived tab
// fetching archived instead.
func startGroupedListsArchiving(t *testing.T, config, state string, height int, archived string) *harness {
	t.Helper()
	transport := groupingTransport(t, "search_grouping.json")
	replyReviewRequests(t, transport, "search_grouping_reviews.json")
	transport.ReplyFixture(t, "PullRequestsByID", fixture(archived))
	dir := t.TempDir()
	if state != "" {
		seedFile(t, dir, "state.json", state)
	}
	h := newHarness(t, transport, withConfigDir(dir), withConfig(config), withTermSize(defaultTermWidth, height))
	h.waitForText("updated 0s ago")
	return h
}

// openReviewRequests switches to the review requests tab and waits for its
// first card to be selected.
func (h *harness) openReviewRequests() string {
	h.t.Helper()
	h.press("tab")
	return h.waitForSelected(r1Ref)
}

// openArchived switches to the archived tab and waits for its first card to
// be selected.
func (h *harness) openArchived() string {
	h.t.Helper()
	h.press("tab")
	h.press("tab")
	return h.waitForSelected(mergedArchivedRef)
}

// tabLine is the row of the tab bar that holds the tab labels.
func tabLine(screen string) string {
	return strings.Split(screen, "\n")[1]
}

func TestReviewRequestsGroupLikeTheBoard(t *testing.T) {
	for _, tc := range []struct {
		grouping string
		order    []string
	}{
		{"owner", []string{"octo-org (3)", r1Ref, r3Ref, r5Ref, "user-b (2)", r2Ref, r4Ref}},
		{"repository", []string{
			"octo-org/review-repo (2)", r1Ref, r5Ref,
			"user-b/sample-repo (1)", r2Ref,
			"octo-org/sample-repo (1)", r3Ref,
			"user-b/tools-repo (1)", r4Ref,
		}},
	} {
		t.Run(tc.grouping, func(t *testing.T) {
			h := startGroupedLists(t, groupingConfig("grouping: "+tc.grouping+"\n"), "", groupingTermHeight)
			screen := h.openReviewRequests()

			assertTopToBottom(t, screen, tc.order...)
			assertContains(t, screen, "grouped by "+tc.grouping)
		})
	}
}

func TestArchivedGroupsLikeTheBoardKeepingFinishedPRsBadged(t *testing.T) {
	for _, tc := range []struct {
		grouping string
		order    []string
	}{
		{"owner", []string{"octo-org (2)", mergedArchivedRef, openArchivedRef, "user-a (1)", closedArchivedRef}},
		{"repository", []string{
			"octo-org/sample-repo (1)", mergedArchivedRef,
			"user-a/other-repo (1)", closedArchivedRef,
			"octo-org/other-repo (1)", openArchivedRef,
		}},
	} {
		t.Run(tc.grouping, func(t *testing.T) {
			h := startGroupedLists(t, groupingConfig("grouping: "+tc.grouping+"\n"), groupingArchivedState, groupingTermHeight)
			screen := h.openArchived()

			assertTopToBottom(t, screen, tc.order...)
			assertContains(t, cardLine(t, screen, mergedArchivedRef, 3), "Merged")
			assertContains(t, cardLine(t, screen, closedArchivedRef, 3), "Closed")
		})
	}
}

func TestListTabCountsAreTheSameWithAndWithoutGrouping(t *testing.T) {
	for _, extra := range []string{"", "grouping: owner\n", "grouping: repository\n"} {
		h := startGroupedLists(t, groupingConfig(extra), groupingArchivedState, groupingTermHeight)
		screen := h.openArchived()

		assertContains(t, tabLine(screen), "Review requests 5", "Archived 3")
		if extra == "" && onScreen(screen, "octo-org (2)") {
			t.Errorf("ungrouped archived list shows a group header:\n%s", screen)
		}
	}
}

func TestCardKeysSkipGroupHeadersInTheLists(t *testing.T) {
	h := startGroupedLists(t, groupingConfig("grouping: owner\n"), groupingArchivedState, groupingTermHeight)

	h.openReviewRequests()
	for _, ref := range []string{r3Ref, r5Ref, r2Ref, r4Ref} {
		h.press("j")
		h.waitForSelected(ref)
	}
	for _, ref := range []string{r2Ref, r5Ref, r3Ref, r1Ref} {
		h.press("k")
		h.waitForSelected(ref)
	}

	h.press("tab")
	h.waitForSelected(mergedArchivedRef)
	for _, ref := range []string{openArchivedRef, closedArchivedRef} {
		h.press("j")
		h.waitForSelected(ref)
	}
	for _, ref := range []string{openArchivedRef, mergedArchivedRef} {
		h.press("k")
		h.waitForSelected(ref)
	}
}

func TestClickingAListGroupHeaderSelectsAndOpensNothing(t *testing.T) {
	h := startGroupedLists(t, groupingConfig("grouping: owner\n"), groupingArchivedState, groupingTermHeight)

	for _, tc := range []struct {
		open           func() string
		header, after  string
		belowTheHeader []string
	}{
		{h.openReviewRequests, "user-b (2)", r3Ref, []string{r2Ref, r4Ref}},
		{func() string { h.press("tab"); return h.waitForSelected(mergedArchivedRef) }, "user-a (1)", openArchivedRef, []string{closedArchivedRef}},
	} {
		screen := tc.open()
		before := len(h.requests())

		x, y, _ := locate(screen, tc.header)
		h.click(x, y)
		h.click(x, y)
		h.press("j")
		screen = h.waitForSelected(tc.after)

		if got := len(h.requests()); got != before {
			t.Errorf("clicking %q sent %d requests", tc.header, got-before)
		}
		for _, ref := range tc.belowTheHeader {
			if selected(screen, ref) {
				t.Errorf("clicking %q selected %s:\n%s", tc.header, ref, screen)
			}
		}
	}
}

func TestArchivingAndUnarchivingLandInTheMatchingGroup(t *testing.T) {
	h := startGroupedListsArchiving(t, groupingConfig("grouping: owner\n"), "", groupingTermHeight, "nodes_grouping_reviews.json")
	h.openReviewRequests()
	h.press("j")
	h.waitForSelected(r3Ref)
	h.press("x")
	h.waitForText("Review requests 4")
	h.press("j")
	h.press("j")
	h.waitForSelected(r4Ref)
	h.press("x")
	screen := h.waitForText("Review requests 3")
	assertTopToBottom(t, screen, "octo-org (2)", r1Ref, r5Ref, "user-b (1)", r2Ref)

	h.press("tab")
	screen = h.waitForText("octo-org (1)")
	assertTopToBottom(t, screen, "octo-org (1)", r3Ref, "user-b (1)", r4Ref)

	h.press("x")
	h.waitForText("Archived 1")
	h.press("shift+tab")
	screen = h.waitForText("octo-org (3)")
	assertTopToBottom(t, screen, "octo-org (3)", r1Ref, r3Ref, r5Ref, "user-b (1)", r2Ref)
}

func TestFilteredPullRequestsAreLeftOutOfListGroups(t *testing.T) {
	config := filterConfig("  excluded_owners: [user-b]\n  repositories:\n    octo-org/other-repo: excluded\n") + "grouping: owner\n"
	h := startGroupedLists(t, config, groupingArchivedState, groupingTermHeight)

	screen := h.openReviewRequests()
	assertTopToBottom(t, screen, "octo-org (3)", r1Ref, r3Ref, r5Ref)
	if strings.Contains(screen, "user-b") {
		t.Errorf("an excluded owner's group or cards are shown:\n%s", screen)
	}
	assertContains(t, tabLine(screen), "Review requests 3")

	h.press("tab")
	screen = h.waitForSelected(mergedArchivedRef)
	assertTopToBottom(t, screen, "octo-org (1)", mergedArchivedRef, "user-a (1)", closedArchivedRef)
	if onScreen(screen, openArchivedRef) {
		t.Errorf("a pull request in an excluded repository is shown:\n%s", screen)
	}
}

func TestLongGroupedListScrollsWithTheSelection(t *testing.T) {
	h := startGroupedLists(t, groupingConfig("grouping: owner\n"), "", 20)
	h.openReviewRequests()

	for range 4 {
		h.press("j")
	}
	screen := h.waitForSelected(r4Ref)
	assertContains(t, screen, "user-b (2)")
	if onScreen(screen, r1Ref) {
		t.Errorf("the list should have scrolled past %s:\n%s", r1Ref, screen)
	}

	for range 4 {
		h.press("k")
	}
	screen = h.waitForSelected(r1Ref)
	assertContains(t, screen, "octo-org (3)")
}

func TestListsGroupByTitlePatternWithNoMatchLast(t *testing.T) {
	config := titlePatternConfig(`title_pattern: '(?i)upload|cache'` + "\n")

	t.Run("review requests", func(t *testing.T) {
		h := startGroupedLists(t, config, "", groupingTermHeight)
		h.press("tab")
		screen := h.waitForSelected(r2Ref)

		assertTopToBottom(t, screen, "upload (1)", r2Ref, "No match (4)", r1Ref, r3Ref, r4Ref, r5Ref)
		assertContains(t, screen, "grouped by title pattern")
	})

	t.Run("archived", func(t *testing.T) {
		h := startGroupedLists(t, config, groupingArchivedState, groupingTermHeight)
		screen := h.openArchived()

		assertTopToBottom(t, screen, "upload (1)", mergedArchivedRef, "cache (2)", closedArchivedRef, openArchivedRef)
	})
}
