package ui_test

import (
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// The reviewers board holds #41, whose reviewers cover every verdict, a
// re-request, a team and a team GitHub won't show; #42, which needs a review
// nobody was asked for; and #43, which needs none.
func newReviewersHarness(t *testing.T, detail string) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_reviewers.json"))
	transport.ReplyFixture(t, "PullRequestDetail", fixture(detail))
	h := newHarness(t, transport, withTermSize(120, 60))
	h.waitForText("octo-org/sample-repo#43")
	return h
}

// rowWith returns the screen line holding every part in order, or fails.
func rowWith(t *testing.T, screen string, parts ...string) string {
	t.Helper()
	for _, l := range strings.Split(screen, "\n") {
		rest, ok := l, true
		for _, p := range parts {
			i := strings.Index(rest, p)
			if i < 0 {
				ok = false
				break
			}
			rest = rest[i+len(p):]
		}
		if ok {
			return l
		}
	}
	t.Fatalf("no line holds %q in order:\n%s", parts, screen)
	return ""
}

func TestStatusListsEachReviewerWithTheirVerdict(t *testing.T) {
	h := newReviewersHarness(t, "detail_reviewers.json")

	h.doubleClickCard("octo-org/sample-repo#41")
	screen := h.waitForText("▸ Conversation")
	rowWith(t, screen, "✗ user-b", "changes requested", "1d ago")
	rowWith(t, screen, "● user-c", "awaiting review", "(approved 3h ago)")
	rowWith(t, screen, "● user-f", "awaiting review")
	rowWith(t, screen, "● @octo-org/infra", "awaiting review")
	rowWith(t, screen, "● a team you can't see", "awaiting review")
	rowWith(t, screen, "✓ user-d", "approved", "2h ago")
	rowWith(t, screen, "○ copilot-reviewer", "commented", "5h ago")
	rowWith(t, screen, "– user-e", "dismissed", "2d ago")
	assertOrder(t, screen,
		"✗ Changes requested",
		"user-b", "user-c", "@octo-org/infra", "user-f", "a team you can't see",
		"user-d", "copilot-reviewer", "user-e",
		"▸ Checks",
	)
}

func TestTheAuthorIsNeverOneOfTheReviewers(t *testing.T) {
	h := newReviewersHarness(t, "detail_reviewers.json")

	h.doubleClickCard("octo-org/sample-repo#41")
	screen := h.waitForText("▸ Conversation")
	for _, l := range strings.Split(screen, "\n") {
		if strings.Contains(l, "user-a") && strings.Contains(l, "commented") {
			t.Errorf("the author's own review is in the roll-call: %q\n%s", l, screen)
		}
	}
}

func TestAHiddenTeamKeepsTheBoardAndModalFresh(t *testing.T) {
	h := newReviewersHarness(t, "detail_reviewers.json")

	h.doubleClickCard("octo-org/sample-repo#41")
	screen := h.waitForText("▸ Conversation")
	for _, bad := range []string{"stale", "Couldn't"} {
		if strings.Contains(screen, bad) {
			t.Errorf("a team GitHub won't show failed the request (%q):\n%s", bad, screen)
		}
	}
}

func TestReviewRequiredWithNoReviewersSaysNobodyWasAsked(t *testing.T) {
	h := newReviewersHarness(t, "detail_no_reviewers.json")

	h.doubleClickCard("octo-org/sample-repo#42")
	screen := h.waitForText("▸ Conversation")
	assertOrder(t, screen, "● Review required", "No reviewers requested", "▸ Checks")
}

func TestNoReviewRequiredAddsNothingWhenNobodyReviewed(t *testing.T) {
	h := newReviewersHarness(t, "detail_no_review_required.json")

	h.doubleClickCard("octo-org/sample-repo#43")
	screen := h.waitForText("▸ Conversation")
	assertContains(t, screen, "No review required")
	if strings.Contains(screen, "No reviewers requested") {
		t.Errorf("a pull request needing no review says nobody was asked:\n%s", screen)
	}
}

func TestYourOwnReviewRequestReadsYou(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_empty.json"))
	replyReviewRequests(t, transport, "search_review_request_of_you.json")
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_review_request_of_you.json"))
	h := newHarness(t, transport, withTermSize(120, 60))
	h.waitForText("Review requests 1")

	h.press("tab")
	h.waitForText("octo-org/sample-repo#44")
	h.press("enter")
	screen := h.waitForText("▸ Conversation")
	rowWith(t, screen, "● you", "awaiting review")
}

func TestPullRequestQueriesAskForReviewRequests(t *testing.T) {
	h := newReviewersHarness(t, "detail_reviewers.json")

	h.doubleClickCard("octo-org/sample-repo#41")
	detail := h.waitForDetailRequests(1)[0]
	for _, r := range []githubtest.Request{h.requests()[0], detail} {
		assertContains(t, r.Query, "reviewRequests", "requestedReviewer", "isViewer")
	}
}
