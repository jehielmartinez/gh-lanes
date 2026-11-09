package ui_test

import (
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// The conversation fixture's timeline, in order, has these headings the
// viewer can fold: the four sections, then user-b's review asking for
// changes and its Dockerfile thread, user-c's review that only opened the
// resolved build.sh thread and that thread, and last the README thread no
// review opened.

func TestAReviewsThreadsSitUnderIt(t *testing.T) {
	h := newConversationHarness(t)
	screen := h.waitForText("Mention the new tag.")

	review, _ := column(t, screen, "✗ user-b requested changes")
	thread, _ := column(t, screen, "▾ Dockerfile:3")
	if thread <= review {
		t.Errorf("the thread a review opened should be indented under it (review at %d, thread at %d):\n%s", review, thread, screen)
	}
	approval, _ := column(t, screen, "✓ user-c approved")
	loose, _ := column(t, screen, "▾ README.md:8")
	if loose != approval {
		t.Errorf("a thread no review opened should stand on its own (entry at %d, thread at %d):\n%s", approval, loose, screen)
	}
}

func TestAReviewThatOnlyOpenedThreadsShowsWithThem(t *testing.T) {
	h := newConversationHarness(t)
	screen := h.waitForText("Mention the new tag.")

	assertOrder(t, screen, "○ user-c reviewed · Mar 3, 2026 10:03 (2d ago)", "▸ scripts/build.sh:12 · Resolved")
}

func TestReviewHeadingsLeadWithTheVerdict(t *testing.T) {
	h := newConversationHarness(t)
	screen := h.waitForText("Mention the new tag.")

	assertContains(t, screen,
		"✗ user-b requested changes · Mar 3, 2026 10:00 (2d ago)",
		"✓ user-c approved · Mar 5, 2026 09:00 (3h ago)",
	)
	if strings.Contains(screen, "[Changes requested]") || strings.Contains(screen, "[Approved]") {
		t.Errorf("the old bracketed badge is still drawn:\n%s", screen)
	}
}

func TestTabHighlightsEachFoldableHeadingInTurn(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Mention the new tag.")

	h.press("tab")
	h.waitForHighlight("Status")
	h.press("tab")
	h.waitForHighlight("Checks")
	h.press("shift+tab")
	h.waitForHighlight("Status")
	if highlighted(h.screen.content(), "Checks") {
		t.Error("two headings are highlighted at once")
	}
}

func TestEnterFoldsTheHighlightedSection(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("pins it by digest.")

	h.tabTo(3)
	h.waitForHighlight("Description")
	h.press("enter")
	h.waitForScreen("the description to close", func(s string) bool {
		return strings.Contains(s, "▸ Description") && !strings.Contains(s, "• Updates the Dockerfile")
	})
	h.press("enter")
	h.waitForText("• Updates the Dockerfile")
}

func TestEnterFoldsTheHighlightedReview(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Mention the new tag.")

	h.tabTo(5)
	h.waitForHighlight("user-b requested changes")
	h.press("enter")
	screen := h.waitForScreen("the review to fold", func(s string) bool {
		return !strings.Contains(s, "Please pin the digest.")
	})
	assertContains(t, screen, "✗ user-b requested changes · Mar 3, 2026 10:00 (2d ago) · ▸ 1 line · 1 thread")
	for _, hidden := range []string{"Dockerfile:3", "Pin this to a digest."} {
		if strings.Contains(screen, hidden) {
			t.Errorf("a folded review still shows %q:\n%s", hidden, screen)
		}
	}
	h.press("enter")
	h.waitForText("Pin this to a digest.")
}

func TestEnterOpensOneResolvedThreadOnly(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Mention the new tag.")

	h.tabTo(8)
	h.waitForHighlight("scripts/build.sh:12")
	h.press("enter")
	h.waitForText("Typo here: biuld.")
	h.press("e")
	h.press("e")
	h.waitForScreen("e to fold every resolved thread again", func(s string) bool {
		return strings.Contains(s, "▸ scripts/build.sh:12") && !strings.Contains(s, "Typo here")
	})
}

func TestTheHighlightScrollsIntoView(t *testing.T) {
	h := newConversationHarness(t)
	h.resize(120, 30)
	// The modal must be too short to show the README thread unscrolled.
	h.waitForScreen("a modal that scrolls", func(s string) bool {
		return strings.Contains(s, "▾ Status") && !strings.Contains(s, "README.md:8")
	})

	h.tabTo(9)
	h.waitForHighlight("README.md:8")
}

func TestClickingAReviewHeadingFoldsIt(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Please pin the digest.")

	h.clickText("user-b requested changes")
	h.waitForScreen("the review to fold", func(s string) bool {
		return !strings.Contains(s, "Please pin the digest.")
	})
	h.waitForHighlight("user-b requested changes")
}

func TestFoldsSurviveARefresh(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Please pin the digest.")

	h.clickText("user-b requested changes")
	h.waitForScreen("the review to fold", func(s string) bool {
		return !strings.Contains(s, "Please pin the digest.")
	})
	n := len(detailRequests(h))
	h.press("r")
	h.waitForDetailRequests(n + 1)
	h.settle(0)
	if strings.Contains(h.screen.plain(), "Please pin the digest.") {
		t.Errorf("a refresh unfolded the review:\n%s", h.screen.plain())
	}
}

func TestHelpListsTheFoldKeys(t *testing.T) {
	h := newConversationHarness(t)

	assertContains(t, h.waitForText("tab highlight"), "enter fold")
}

func newCopilotHarness(t *testing.T) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_copilot.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#12")
	h.resize(120, 80)
	h.press("enter")
	h.expandSections()
	return h
}

func TestCopilotReviewsReadCleanly(t *testing.T) {
	h := newCopilotHarness(t)
	screen := h.waitForText("Review effort: Balanced")

	assertOrder(t, screen,
		"○ Copilot reviewed · Mar 5, 2026 09:00 (3h ago)",
		"Note", "Copilot was unable to run its full agentic suite in this review.",
		"🟡 Changes recommended",
		"Pinning by digest can leave the cache step",
		"▾ 2 open issues",
		"Medium severity", "Refresh the cache key when the digest changes",
		"Low severity", "The build arguments aren't passed to the second stage",
		"▸ What changed in this PR",
		"Review effort: Balanced",
		"▾ Dockerfile:3 · Unresolved",
		"Copilot · Mar 5, 2026 08:59 (3h ago)",
		"The cache key ignores the digest",
		"Suggested change", "ARG CACHE_KEY=${BASE_DIGEST}",
	)
	for _, raw := range []string{
		"copilot-pull-request-reviewer", "[!NOTE]", "<details", "<summary", "<picture", "<img", "<sub>",
		"&#39;", "```suggestion",
		// A closed <details> keeps its contents folded, as GitHub does.
		"Pins the base image by digest.", "| File",
	} {
		if strings.Contains(screen, raw) {
			t.Errorf("the Copilot review shows %q:\n%s", raw, screen)
		}
	}
	rowWith(t, screen, "○ Copilot", "commented")
	// An alert's kind heads it on a line of its own.
	_, kind := column(t, screen, "Note")
	_, text := column(t, screen, "Copilot was unable")
	if kind == text {
		t.Errorf("the alert's kind shares a line with its text:\n%s", screen)
	}
}

func TestEncodedControlCharactersStayText(t *testing.T) {
	h := newCopilotHarness(t)
	screen := h.waitForText("Encoded escapes stay text:")

	assertContains(t, screen, "&#27;[2J &#x1b;]0;title&#7; done.")
	raw := h.screen.content()
	for _, seq := range []string{"\x1b[2J", "\x1b]0;", "\x07"} {
		if strings.Contains(raw, seq) {
			t.Errorf("an encoded %q reached the terminal", seq)
		}
	}
}

func TestCopilotIssueLinksAreClickable(t *testing.T) {
	h := newCopilotHarness(t)
	h.waitForText("Refresh the cache key when the digest changes")

	h.clickText("Refresh the cache key")
	h.waitForOpened("https://github.com/user-a/other-repo/pull/13#discussion_r1")
}

// tabTo presses tab n times, highlighting the nth foldable heading.
func (h *harness) tabTo(n int) {
	h.t.Helper()
	for range n {
		h.press("tab")
	}
}

// waitForHighlight waits until text is drawn highlighted.
func (h *harness) waitForHighlight(text string) {
	h.t.Helper()
	h.waitFor(text+" highlighted", func() bool { return highlighted(h.screen.content(), text) })
}

// highlighted is whether text starts right after an SGR sequence that turns
// reverse video on.
func highlighted(raw, text string) bool {
	for _, seq := range styling.FindAllStringIndex(raw, -1) {
		if !strings.HasPrefix(raw[seq[1]:], text) || raw[seq[1]-1] != 'm' {
			continue
		}
		for _, p := range strings.FieldsFunc(raw[seq[0]+2:seq[1]-1], func(r rune) bool { return r == ';' }) {
			if p == "7" {
				return true
			}
		}
	}
	return false
}

// column is where text starts on the plain screen, or fails.
func column(t *testing.T, screen, text string) (x, y int) {
	t.Helper()
	x, y, ok := locate(screen, text)
	if !ok {
		t.Fatalf("%q is not on screen:\n%s", text, screen)
	}
	return x, y
}
