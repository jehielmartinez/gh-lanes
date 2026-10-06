package ui_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

func TestBoardShowsMyOpenPRsAsCardsInUntaggedLane(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport)

	screen := h.waitForText("octo-org/sample-repo#7")

	assertContains(t, screen,
		"Untagged 3",
		"user-a/other-repo#42",
		"Add retry with exponential back…",
		"octo-org/sample-repo#7",
		"Fix typo in README",
		"octo-org/a-very-long-sample-repository-name#1234",
	)
	// The owner and number share the first line, the number at the right
	// edge; the repository has the second line, truncated when it is long.
	long := "octo-org/a-very-long-sample-repository-name#1234"
	if first := cardLine(t, screen, long, 0); !strings.HasPrefix(first, "octo-org ") || !strings.HasSuffix(strings.TrimRight(first, " "), "#1234") {
		t.Errorf("first line = %q, want the owner and then the number at the right", first)
	}
	if second := strings.TrimSpace(cardLine(t, screen, long, 1)); second != "a-very-long-sample-repository-n…" {
		t.Errorf("second line = %q, want the repository truncated to fit", second)
	}
	if strings.Contains(screen, "upload worker queue") {
		t.Errorf("long title was not truncated:\n%s", screen)
	}
	if strings.Index(screen, "#42") > strings.Index(screen, "#7") {
		t.Errorf("most recently updated PR should be first:\n%s", screen)
	}
}

func TestBoardRunsTheAuthoredOpenPRSearch(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")

	reqs := h.requests()
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	if got := reqs[0].Operation; got != "SearchPullRequests" {
		t.Errorf("operation = %q, want SearchPullRequests", got)
	}
	if got := reqs[0].Variables["query"]; got != "is:pr is:open author:@me archived:false" {
		t.Errorf("search = %q", got)
	}
}

func TestBoardPagesThroughEverySearchResult(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_page_1.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_page_2.json"))
	h := newHarness(t, transport)

	screen := h.waitForText("octo-org/sample-repo#2")
	if !onScreen(screen, "octo-org/sample-repo#1") || !strings.Contains(screen, "Untagged 2") {
		t.Errorf("both pages should be on the board:\n%s", screen)
	}

	reqs := h.requests()
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if got := reqs[0].Variables["after"]; got != nil {
		t.Errorf("first page after = %v, want null", got)
	}
	if got := reqs[1].Variables["after"]; got != "Y3Vyc29yOjE=" {
		t.Errorf("second page after = %v, want the first page's end cursor", got)
	}
}

func TestBoardWithNoOpenPRsSaysSo(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_empty.json"))
	h := newHarness(t, transport)

	screen := h.waitForText("No open pull requests.")
	if !strings.Contains(screen, "Untagged 0") {
		t.Errorf("lane header should show a zero count:\n%s", screen)
	}
}

func TestBoardStripsEscapeSequencesFromTitles(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_hostile_title.json"))
	h := newHarness(t, transport)

	h.waitForText("Fix bug now")
	raw := h.screen.content()
	for _, bad := range []string{"\x1b[2J", "\x1b]0;", "pwned", "\u009b", "\x07"} {
		if strings.Contains(raw, bad) {
			t.Errorf("rendered screen contains %q from the PR title", bad)
		}
	}
}

func TestBoardShowsLoadErrorInStatusBar(t *testing.T) {
	transport := githubtest.New()
	transport.Reply("SearchPullRequests", githubtest.Response{
		Status: 502,
		Body:   []byte(`{"message":"Server Error"}`),
	})
	h := newHarness(t, transport)

	screen := h.waitForText("Couldn't load pull requests")
	if strings.Contains(screen, "No open pull requests.") {
		t.Errorf("a failed load must not look like an empty board:\n%s", screen)
	}
}

func TestBoardShowsLoadingUntilTheSearchAnswers(t *testing.T) {
	body, err := os.ReadFile(fixture("search_board.json"))
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	transport := githubtest.New()
	transport.Reply("SearchPullRequests", githubtest.Response{Body: body, Release: release})
	h := newHarness(t, transport)

	h.waitForText("Loading pull requests…")
	close(release)
	screen := h.waitForText("octo-org/sample-repo#7")
	if strings.Contains(screen, "Loading pull requests") {
		t.Errorf("loading message should clear once the board loads:\n%s", screen)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			transport := githubtest.New()
			transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
			h := newHarness(t, transport)
			h.waitForText("octo-org/sample-repo#7")

			h.press(key)
			h.waitFinished()
		})
	}
}
