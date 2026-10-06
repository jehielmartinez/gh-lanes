package ui_test

import (
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// newConversationHarness starts the app with the conversation fixtures queued,
// opens the most recently updated card and waits for its conversation.
func newConversationHarness(t *testing.T) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_conversation.json"))
	transport.ReplyFixture(t, "PullRequestComments", fixture("conversation_comments_page_2.json"))
	transport.ReplyFixture(t, "PullRequestReviews", fixture("conversation_reviews_page_2.json"))
	transport.ReplyFixture(t, "PullRequestReviewThreads", fixture("conversation_threads_page_2.json"))
	transport.ReplyFixture(t, "ReviewThreadComments", fixture("thread_comments_page_2.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#12")
	h.resize(120, 120)
	h.press("enter")
	h.waitForText("Conversation")
	return h
}

func TestDescriptionIsRenderedAsMarkdown(t *testing.T) {
	h := newConversationHarness(t)

	screen := h.waitForText("Description")
	assertContains(t, screen,
		"Summary",
		"Moves the base image to alpine 3.21 and pins it by digest.",
		"• Updates the Dockerfile",
		"• Drops the old cache step",
		"• keeps the build arguments",
		"1. Build the image",
		"2. Push it to the registry",
		"docker build -t sample .",
		"Build notes live at https://github.com/octo-org/sample-repo/wiki/Build.",
	)
	assertOrder(t, screen, "Description", "Summary", "Moves the base image", "• Updates", "1. Build", "docker build", "Build notes")
	for _, raw := range []string{"## Summary", "**alpine", "](https://", "`Dockerfile`", "```", "<!--", "Describe the change"} {
		if strings.Contains(screen, raw) {
			t.Errorf("markdown source %q shown as text:\n%s", raw, screen)
		}
	}
}

func TestMarkdownLinksAreUnderlinedWithoutTerminalHyperlinks(t *testing.T) {
	h := newConversationHarness(t)

	h.waitForText("pins it by digest.")
	raw := h.screen.content()
	if !strings.Contains(raw, "digest") || !underlined(raw, "digest") {
		t.Errorf("the link text is not underlined:\n%q", raw)
	}
	if strings.Contains(raw, "\x1b]8;") {
		t.Error("the modal emitted an OSC 8 hyperlink")
	}
}

// underlined is whether text is drawn right after an SGR sequence that turns
// underline on.
func underlined(raw, text string) bool {
	for _, seq := range styling.FindAllStringIndex(raw, -1) {
		params := raw[seq[0]+2 : seq[1]-1]
		if !strings.HasPrefix(raw[seq[1]:], text) || raw[seq[1]-1] != 'm' {
			continue
		}
		for _, p := range strings.FieldsFunc(params, func(r rune) bool { return r == ';' }) {
			if p == "4" || strings.HasPrefix(p, "4:") {
				return true
			}
		}
	}
	return false
}

func TestConversationIsOneTimelineSortedByTime(t *testing.T) {
	h := newConversationHarness(t)

	screen := h.waitForText("Mention the new tag.")
	assertOrder(t, screen,
		"Conversation",
		"user-b commented · Mar 2, 2026 10:00 (3d ago)", "Looks close. One question about the cache.",
		"user-b [Changes requested] · Mar 3, 2026 10:00 (2d ago)", "Please pin the digest.",
		"▾ Dockerfile:3 · Unresolved · Mar 3, 2026 10:01 (2d ago)",
		"user-b · Mar 3, 2026 10:01 (2d ago)", "Pin this to a digest.",
		"user-a · Mar 4, 2026 11:00 (1d ago)", "Done in the next push.",
		"▸ scripts/build.sh:12 · Resolved · 2 comments",
		"user-a commented · Mar 4, 2026 10:00 (1d ago)", "Rebased onto main.",
		"user-c [Approved] · Mar 5, 2026 09:00 (3h ago)",
		"▾ README.md:8 · Unresolved · Mar 5, 2026 09:05 (2h ago)",
		"ghost · Mar 5, 2026 09:05 (2h ago)", "Mention the new tag.",
	)
	// user-c's empty COMMENTED review only carried thread comments.
	if n := strings.Count(screen, "user-c"); n != 1 {
		t.Errorf("want only user-c's approval in the timeline, found user-c %d times:\n%s", n, screen)
	}
}

func TestResolvedThreadsAreCollapsedUntilExpanded(t *testing.T) {
	h := newConversationHarness(t)

	screen := h.waitForText("▸ scripts/build.sh:12 · Resolved · 2 comments")
	for _, hidden := range []string{"Typo here: biuld.", "Fixed in the next push."} {
		if strings.Contains(screen, hidden) {
			t.Errorf("collapsed thread shows %q:\n%s", hidden, screen)
		}
	}

	h.press("e")
	screen = h.waitForText("Fixed in the next push.")
	assertOrder(t, screen,
		"▾ scripts/build.sh:12 · Resolved · Mar 3, 2026 10:02 (2d ago)",
		"user-b · Mar 3, 2026 10:02 (2d ago)", "Typo here: biuld.",
		"user-a · Mar 4, 2026 10:30 (1d ago)", "Fixed in the next push.",
	)

	h.press("e")
	h.waitForScreen("the resolved thread to collapse again", func(s string) bool {
		return strings.Contains(s, "▸ scripts/build.sh:12") && !strings.Contains(s, "Typo here")
	})
}

func TestEveryConversationConnectionIsPagedThrough(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Mention the new tag.")

	want := map[string]struct{ id, after string }{
		"PullRequestComments":      {"PR_node_behind", "Y29tbWVudHM6MQ=="},
		"PullRequestReviews":       {"PR_node_behind", "cmV2aWV3czoy"},
		"PullRequestReviewThreads": {"PR_node_behind", "dGhyZWFkczoy"},
		"ReviewThreadComments":     {"RT_node_2", "dGhyZWFkLWNvbW1lbnRzOjE="},
	}
	got := map[string]int{}
	for _, r := range h.transport.Requests() {
		w, ok := want[r.Operation]
		if !ok {
			continue
		}
		got[r.Operation]++
		if r.Variables["id"] != w.id || r.Variables["after"] != w.after {
			t.Errorf("%s asked for id %v after %v, want %s after %s", r.Operation, r.Variables["id"], r.Variables["after"], w.id, w.after)
		}
	}
	for op := range want {
		if got[op] != 1 {
			t.Errorf("want one %s request, got %d", op, got[op])
		}
	}
}

func TestConversationTextHasEscapeSequencesStripped(t *testing.T) {
	h := newConversationHarness(t)

	screen := h.waitForText("Danger zone.")
	assertContains(t, screen, "Rebased onto main. Cache stays.")
	if strings.Contains(screen, "pwned") {
		t.Errorf("an escape sequence's payload reached the screen:\n%s", screen)
	}
	raw := h.screen.content()
	for _, seq := range []string{"\x1b]0;", "\x1b[2J", "\x07"} {
		if strings.Contains(raw, seq) {
			t.Errorf("the screen carries %q from PR text", seq)
		}
	}
}

func TestConversationQueriesAskForNoCodeOrDiffs(t *testing.T) {
	h := newConversationHarness(t)
	h.waitForText("Mention the new tag.")

	for _, r := range h.transport.Requests() {
		for _, bad := range []string{"diffHunk", "files", "patch", "originalCommit"} {
			if strings.Contains(r.Query, bad) {
				t.Errorf("%s asks for %q", r.Operation, bad)
			}
		}
		if r.Operation == "PullRequestDetail" {
			assertContains(t, r.Query, "body", "reviewThreads", "isResolved", "originalLine")
		}
	}
}

func TestEmptyConversationSaysSo(t *testing.T) {
	h := newModalHarness(t, "detail_behind.json")
	h.resize(120, 80)

	h.press("enter")
	screen := h.waitForText("No comments yet.")
	assertContains(t, screen, "Description", "No description provided.", "Conversation  0")
}
