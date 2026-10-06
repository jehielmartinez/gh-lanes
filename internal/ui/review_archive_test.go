package ui_test

import (
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

const openReviewPR = "PR_node_review_open"

// archivedOpenReview is the state file entry for the open review request
// (#31), archived from review requests while it was requested.
var archivedOpenReview = archivedJSON{ID: openReviewPR, Open: true, Origin: "review_requests"}

// reviewArchiveTransport answers the board search with the board fixture,
// each review requests search with the next of reviews (the last repeating),
// and a fetch by ID with the open review request.
func reviewArchiveTransport(t *testing.T, reviews ...string) *githubtest.Transport {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	for _, r := range reviews {
		replyReviewRequests(t, transport, r)
	}
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_review_branch_updated.json"))
	return transport
}

// startWithArchivedReview starts the app with the open review request
// archived from review requests, and waits until the board is on screen.
func startWithArchivedReview(t *testing.T, open bool, reviews ...string) *harness {
	t.Helper()
	entry := `{"id":"PR_node_review_open","open":false,"origin":"review_requests"}`
	if open {
		entry = `{"id":"PR_node_review_open","open":true,"origin":"review_requests"}`
	}
	return startWithState(t, reviewArchiveTransport(t, reviews...), `{"version":1,"assignments":{},"archived":[`+entry+`]}`, "Untagged 3")
}

func TestArchivingAReviewRequestMovesItToArchived(t *testing.T) {
	h := newHarness(t, reviewArchiveTransport(t, "search_review_requests.json"))
	h.waitForText("Review requests 2")
	h.press("tab")
	h.waitForText(openReviewRef)
	h.press("?")
	h.waitForText("x archive")
	h.press("?")

	h.press("j")
	h.press("x")
	h.waitForArchived([]archivedJSON{archivedOpenReview})
	screen := h.waitForText("Review requests 1")
	assertContains(t, screen, "Archived 1", lockedReviewRef)
	if onScreen(screen, openReviewRef) {
		t.Errorf("the archived review request should leave review requests:\n%s", screen)
	}
	if s, _ := readState(h.configDir); len(s.Assignments) != 0 {
		t.Errorf("archiving a review request should assign no lane: %v", s.Assignments)
	}

	h.press("tab")
	screen = h.waitForText(openReviewRef)
	assertContains(t, screen, "review request", "Add pagination to the export API")
}

func TestAnArchivedReviewRequestStaysOffWhileStillRequested(t *testing.T) {
	h := startWithArchivedReview(t, true, "search_review_requests.json")
	h.waitForText("Review requests 1")

	h.press("r")
	h.waitFor("a second review requests search", func() bool { return reviewSearches(h) == 2 })
	h.waitForText("Review requests 1")
	h.waitForArchived([]archivedJSON{archivedOpenReview})
}

func TestUnarchivingAReviewRequestSendsItBackToReviewRequests(t *testing.T) {
	h := startWithArchivedReview(t, true, "search_review_requests.json")

	h.clickText("Archived 1")
	h.waitForText(openReviewRef)
	h.press("x")
	h.waitForText("restored to Review requests")
	h.waitForArchived(nil)
	screen := h.waitForText("Review requests 2")
	assertContains(t, screen, "Archived 0")

	h.press("shift+tab")
	h.waitForText(openReviewRef)
	h.press("shift+tab")
	screen = h.waitForText("Untagged 3")
	if strings.Contains(screen, "Add pagination to the export API") {
		t.Errorf("an unarchived review request must not join the board:\n%s", screen)
	}
	if s, _ := readState(h.configDir); len(s.Assignments) != 0 {
		t.Errorf("unarchiving a review request should assign no lane: %v", s.Assignments)
	}
}

func TestUnarchivingAReviewRequestNoLongerRequestedSaysItStaysOff(t *testing.T) {
	h := startWithArchivedReview(t, false, "search_empty.json")

	h.clickText("Archived 1")
	h.waitForText(openReviewRef)
	h.press("x")
	h.waitForText("your review is no longer requested, so it stays off Review requests")
	h.waitForArchived(nil)
	h.waitForText("Nothing archived.")
	h.press("shift+tab")
	h.waitForText("No review requests.")
}

func TestAReRequestedReviewComesBackFromTheArchive(t *testing.T) {
	h := startWithArchivedReview(t, true, "search_review_requests.json", "search_empty.json", "search_review_requests.json")
	h.waitForText("Review requests 1")

	h.press("r")
	h.waitForArchived([]archivedJSON{{ID: openReviewPR, Open: false, Origin: "review_requests"}})
	h.waitForText("Review requests 0")

	h.press("r")
	h.waitForArchived(nil)
	screen := h.waitForText("Review requests 2")
	assertContains(t, screen, "Archived 0")
}
