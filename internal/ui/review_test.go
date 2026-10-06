package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// The review requests hold, newest first, a pull request the login may not
// change (#32) and one whose branch is behind and can be updated (#31).
const (
	lockedReviewRef = "octo-org/tools-repo#32"
	openReviewRef   = "octo-org/sample-repo#31"
)

// newReviewHarness starts the app on the board fixture with two review
// requests. reply queues extra replies before the app starts.
func newReviewHarness(t *testing.T, reply func(*githubtest.Transport)) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	replyReviewRequests(t, transport, "search_review_requests.json")
	if reply != nil {
		reply(transport)
	}
	h := newHarness(t, transport)
	h.waitForText("Review requests 2")
	return h
}

func reviewSearches(h *harness) int {
	n := 0
	for _, r := range h.transport.Requests() {
		if isReviewSearch(r) {
			n++
		}
	}
	return n
}

func TestReviewRequestsAreSearchedWithEveryRefresh(t *testing.T) {
	h := newReviewHarness(t, nil)
	if n := reviewSearches(h); n != 1 {
		t.Fatalf("want the review requests search on load, got %d", n)
	}

	h.press("r")
	h.waitFor("a second review requests search", func() bool { return reviewSearches(h) == 2 })
}

func TestTabsSwitchBetweenTheBoardAndReviewRequests(t *testing.T) {
	h := newReviewHarness(t, nil)

	screen := h.screen.plain()
	// The labels sit in the middle row of the tab bar's boxes.
	if tabs := strings.Split(screen, "\n")[1]; !strings.Contains(tabs, "Board") || !strings.Contains(tabs, "Review requests 2") {
		t.Errorf("the tabs should be the top line:\n%s", screen)
	}
	assertContains(t, screen, "Untagged 3", "octo-org/sample-repo#7")

	h.press("tab")
	screen = h.waitForText(lockedReviewRef)
	assertContains(t, screen, openReviewRef, "Speed up the lint step", "Add pagination to the export API")
	assertOrder(t, screen, lockedReviewRef, openReviewRef)
	for _, lane := range []string{"Untagged", "In Progress", "octo-org/sample-repo#7"} {
		if strings.Contains(screen, lane) {
			t.Errorf("review requests are one list with no lanes, but %q is on screen:\n%s", lane, screen)
		}
	}

	h.press("shift+tab")
	screen = h.waitForText("Untagged 3")
	if onScreen(screen, lockedReviewRef) {
		t.Errorf("review requests should leave the board's lanes:\n%s", screen)
	}
}

func TestReviewRequestsNeverJoinTheBoardsLanes(t *testing.T) {
	h := newReviewHarness(t, nil)

	screen := h.screen.plain()
	if onScreen(screen, lockedReviewRef) || onScreen(screen, openReviewRef) {
		t.Errorf("review requests are not mine and must stay off the board:\n%s", screen)
	}
	assertContains(t, headerLine(screen), "Untagged 3")
}

func TestNoReviewRequestsSaysSo(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")

	h.press("tab")
	h.waitForText("No review requests.")
}

func TestReviewRequestsOpenInTheDetailModal(t *testing.T) {
	h := newReviewHarness(t, func(tr *githubtest.Transport) {
		tr.Reply("PullRequestDetail", githubtest.Response{Err: errors.New("dial tcp: network is unreachable")})
	})

	h.press("tab")
	h.waitForText(lockedReviewRef)
	h.press("j")
	h.press("enter")
	reqs := h.waitForDetailRequests(1)
	if got := reqs[0].Variables["id"]; got != "PR_node_review_open" {
		t.Errorf("detail fetched for %v, want the selected review request", got)
	}
	screen := h.waitForText("export-pages → main")
	assertContains(t, screen, "user-b", "Behind main. Update branch available.", "▾ Status", "Checks")

	h.press("esc")
	h.waitForScreen("the review requests again", func(s string) bool {
		return !strings.Contains(s, "▾ Status") && onScreen(s, openReviewRef)
	})
}

func TestDoubleClickOpensAReviewRequest(t *testing.T) {
	h := newReviewHarness(t, nil)
	h.press("tab")
	h.waitForText(openReviewRef)

	h.doubleClickCard(openReviewRef)
	reqs := h.waitForDetailRequests(1)
	if got := reqs[0].Variables["id"]; got != "PR_node_review_open" {
		t.Errorf("detail fetched for %v, want the double-clicked review request", got)
	}
}

func TestClickingATabSwitchesToIt(t *testing.T) {
	h := newReviewHarness(t, nil)

	h.clickText("Review requests 2")
	h.waitForText(lockedReviewRef)
	h.clickText("Board")
	h.waitForText("Untagged 3")
}

func TestActionsOnReviewRequestsFollowWhatTheLoginMayChange(t *testing.T) {
	h := newReviewHarness(t, nil)
	h.press("tab")
	h.waitForText(lockedReviewRef)

	// The login can't change #32, though its branch is behind.
	h.press("?")
	screen := h.waitForText("close help")
	for _, hidden := range []string{"update branch", "rebase branch", "convert to draft", "ready for review", "move to", "archive"} {
		if strings.Contains(screen, hidden) {
			t.Errorf("%q should not be offered for #32:\n%s", hidden, screen)
		}
	}
	h.press("u")
	h.press("U")
	h.press("d")
	h.press("x")
	h.press("H")
	assertNoMutations(t, h)
	if s := h.screen.plain(); strings.Contains(s, "Rebase branch?") || strings.Contains(s, "Convert to draft?") {
		t.Errorf("no dialog should open for an action that isn't offered:\n%s", s)
	}
	assertContains(t, h.screen.plain(), lockedReviewRef, openReviewRef)

	// The login may change #31.
	h.press("j")
	h.waitForText("u update branch")
}

func TestUpdateBranchWorksOnAReviewRequest(t *testing.T) {
	h := newReviewHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(updateBranchOp, githubtest.Response{Body: []byte(`{"data":{"updatePullRequestBranch":{"pullRequest":{"id":"PR_node_review_open"}}}}`)})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_review_branch_updated.json"))
	})
	h.press("tab")
	h.waitForText(lockedReviewRef)

	h.press("j")
	h.press("u")
	assertInput(t, h.waitForMutation(updateBranchOp), map[string]any{
		"pullRequestId": "PR_node_review_open",
		"updateMethod":  "MERGE",
	})
	screen := h.waitForScreen("the refreshed review request on top", func(s string) bool {
		return strings.Contains(s, openReviewRef+": branch updated") && !strings.Contains(cardStatus(t, s, openReviewRef), "behind")
	})
	assertOrder(t, screen, openReviewRef, lockedReviewRef)
}

func TestAFailedReviewSearchKeepsBothTabsMarkedStale(t *testing.T) {
	h := newReviewHarness(t, func(tr *githubtest.Transport) {
		tr.ReplyWhen("SearchPullRequests", "query", reviewSearch, githubtest.Response{Status: 502, Body: []byte(`{"message":"Server Error"}`)})
	})
	h.press("tab")
	h.waitForText(lockedReviewRef)

	h.press("r")
	screen := h.waitForText("stale")
	assertContains(t, screen, "Couldn't refresh: review requests:", "HTTP 502", lockedReviewRef, openReviewRef, "Review requests 2")

	h.press("shift+tab")
	screen = h.waitForText("Untagged 3")
	assertContains(t, screen, "stale", "octo-org/sample-repo#7")
}

func TestOpeningAReviewRequestRecordsNoSnapshot(t *testing.T) {
	h := newReviewHarness(t, func(tr *githubtest.Transport) {
		tr.Reply("PullRequestDetail", githubtest.Response{Err: errors.New("dial tcp: network is unreachable")})
	})
	h.waitForSnapshots(newestPR)
	h.press("tab")
	h.waitForText(lockedReviewRef)

	h.press("enter")
	h.waitForText("network is unreachable")
	// A snapshot would be written as the modal opens; give it time to land.
	time.Sleep(50 * time.Millisecond)
	snaps, _ := readSnapshots(h.configDir)
	if _, ok := snaps["PR_node_review_locked"]; ok {
		t.Errorf("a review request is not on the board and should have no snapshot: %v", snaps)
	}
	if strings.Contains(h.screen.plain(), "new PR") {
		t.Errorf("review requests should not count as new:\n%s", h.screen.plain())
	}
}

func TestShiftOOpensTheSelectedReviewRequest(t *testing.T) {
	h := newReviewHarness(t, nil)
	h.press("tab")
	h.waitForText(openReviewRef)

	h.press("j")
	h.press("O")
	h.waitForOpened("https://github.com/octo-org/sample-repo/pull/31")
}

func TestLinkPickerLoadsTheSelectedReviewRequest(t *testing.T) {
	h := newReviewHarness(t, func(tr *githubtest.Transport) {
		tr.Reply("PullRequestDetail", githubtest.Response{Err: errors.New("connection reset")})
	})
	h.press("tab")
	h.waitForText(openReviewRef)

	h.press("j")
	h.press("o")
	reqs := h.waitForDetailRequests(1)
	if got := reqs[0].Variables["id"]; got != "PR_node_review_open" {
		t.Errorf("links loaded for %v, want the selected review request", got)
	}
}

func TestReviewRequestsHelpListsTheLinkKeys(t *testing.T) {
	h := newReviewHarness(t, nil)
	h.press("tab")
	h.waitForText(openReviewRef)

	h.press("?")
	h.waitForText("open in browser")
	h.waitForText("links")
}
