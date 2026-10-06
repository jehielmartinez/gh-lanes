package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// newModalHarness starts the app on the status board, with the given detail
// replies queued, and waits for the board.
func newModalHarness(t *testing.T, details ...string) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	for _, d := range details {
		transport.ReplyFixture(t, "PullRequestDetail", fixture(d))
	}
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#12")
	return h
}

// cardAt returns a screen cell inside the card whose first line holds ref.
func cardAt(t *testing.T, screen, ref string) (x, y int) {
	t.Helper()
	for i, l := range strings.Split(screen, "\n") {
		if at := strings.Index(l, ref); at >= 0 {
			return lipgloss.Width(l[:at]), i
		}
	}
	t.Fatalf("no card for %s:\n%s", ref, screen)
	return 0, 0
}

func (h *harness) doubleClickCard(ref string) {
	h.t.Helper()
	x, y := cardAt(h.t, h.screen.plain(), ref)
	h.click(x, y)
	h.click(x, y)
}

func detailRequests(h *harness) []githubtest.Request {
	var reqs []githubtest.Request
	for _, r := range h.transport.Requests() {
		if r.Operation == "PullRequestDetail" {
			reqs = append(reqs, r)
		}
	}
	return reqs
}

func (h *harness) waitForDetailRequests(n int) []githubtest.Request {
	h.t.Helper()
	h.waitFor("detail requests", func() bool { return len(detailRequests(h)) >= n })
	return detailRequests(h)
}

func assertContains(t *testing.T, screen string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(screen, w) {
			t.Errorf("screen is missing %q:\n%s", w, screen)
		}
	}
}

// assertOrder checks that each string appears, in order, on the screen.
func assertOrder(t *testing.T, screen string, order ...string) {
	t.Helper()
	last := -1
	for _, s := range order {
		i := strings.Index(screen, s)
		if i < 0 || i < last {
			t.Fatalf("want %v in that order:\n%s", order, screen)
		}
		last = i
	}
}

func TestEnterOpensSelectedPullRequestFetchedFresh(t *testing.T) {
	h := newModalHarness(t, "detail_behind.json")

	h.press("enter")
	reqs := h.waitForDetailRequests(1)
	if got := reqs[0].Variables["id"]; got != "PR_node_behind" {
		t.Errorf("detail fetched for %v, want the most recently updated card", got)
	}
	// Only the fresh copy has no checks; the board's has one running.
	screen := h.waitForText("No checks")
	assertContains(t, screen,
		"user-a/other-repo#13", "Open", "Bump the base image", "user-a", "feature-13 → main",
		"Behind main. Update branch available.", "Changes requested", "Auto-merge", "Off",
	)
}

func TestDoubleClickOpensTheCardUnderThePointer(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")

	h.doubleClickCard("octo-org/sample-repo#11")
	reqs := h.waitForDetailRequests(1)
	if got := reqs[0].Variables["id"]; got != "PR_node_conflict" {
		t.Errorf("detail fetched for %v, want the double-clicked card", got)
	}
	h.waitForText("retry-uploads → release-2")
}

func TestSingleClicksDoNotOpenTheModal(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")

	x, y := cardAt(t, h.screen.plain(), "octo-org/sample-repo#11")
	h.click(x, y)
	// The first click has no visible effect to wait for, so give it time to
	// be handled before the clock moves.
	time.Sleep(50 * time.Millisecond)
	h.advance(time.Second)
	h.click(x, y)
	time.Sleep(50 * time.Millisecond)
	if n := len(detailRequests(h)); n != 0 {
		t.Fatalf("clicks a second apart opened the modal (%d detail requests)", n)
	}
}

func TestHeaderShowsStateAuthorBranchesAndLocalTimes(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")
	h.resize(120, 60)

	h.doubleClickCard("octo-org/sample-repo#11")
	screen := h.waitForText("retry-uploads → release-2")
	assertContains(t, screen,
		"octo-org/sample-repo#11", "Open", "Add retry to the upload worker", "user-a",
		"Created Feb 1, 2026 05:00 (1mo ago)",
		"Updated Mar 5, 2026 09:00 (3h ago)",
	)
}

func TestMergeStatusNamesTheActualBaseBranch(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")
	h.resize(120, 60)

	h.doubleClickCard("octo-org/sample-repo#11")
	screen := h.waitForText("Conflicts with release-2. Resolve them before merging.")
	assertContains(t, screen,
		"Approved",
		"Auto-merge", "Squash when ready, enabled by user-a on Mar 5, 2026 10:00 (2h ago)",
	)
}

func TestUnknownMergeabilityInTheModalIsChecking(t *testing.T) {
	h := newModalHarness(t, "detail_checking.json")

	h.doubleClickCard("octo-org/sample-repo#12")
	screen := h.waitForText("Checking whether this can merge into main…")
	for _, bad := range []string{"Conflict", "Ready to merge"} {
		if strings.Contains(screen, bad) {
			t.Errorf("UNKNOWN mergeability shown as %q:\n%s", bad, screen)
		}
	}
	assertContains(t, screen, "Review required")
}

func TestChecksAreGroupedWithFailuresFirst(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")
	h.resize(120, 60)

	h.doubleClickCard("octo-org/sample-repo#11")
	screen := h.waitForText("retry-uploads → release-2")
	assertContains(t, screen,
		"5 passed · 1 failed · 3 running · 1 skipped",
		"✗ integration", "1h 12m",
		"● e2e", "running 15m 0s",
		"● smoke", "queued",
		"✓ unit", "7m 30s",
		"✓ build", "4m 10s",
		"✓ lint", "42s",
		"✓ ci/external", "3h ago",
		"● deploy/preview", "30m ago",
	)
	assertOrder(t, screen,
		"Test", "✗ integration", "● e2e", "● smoke", "✓ unit", "docs",
		"Commit statuses", "● deploy/preview", "✓ ci/external",
		"Build", "✓ build", "✓ lint",
		"Other checks", "✓ coverage-report",
	)
}

func TestModalChecksArePagedThrough(t *testing.T) {
	h := newModalHarness(t, "detail_paged.json")
	h.resize(120, 60)

	h.doubleClickCard("user-a/other-repo#14")
	screen := h.waitForText("unit-3")
	assertContains(t, screen, "1 passed · 2 failed")
	var paged int
	for _, r := range h.transport.Requests() {
		if r.Operation == "CheckContexts" && r.Variables["id"] == "C_node_paged" {
			paged++
		}
	}
	if paged != 2 {
		t.Errorf("want the head commit's checks paged for the board and the modal, got %d requests", paged)
	}
}

func TestModalScrollsAsOneViewport(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")

	h.doubleClickCard("octo-org/sample-repo#11")
	screen := h.waitForText("retry-uploads → release-2")
	if strings.Contains(screen, "coverage-report") {
		t.Fatalf("a 30-line terminal should not fit every check:\n%s", screen)
	}
	h.press("end")
	screen = h.waitForText("coverage-report")
	if strings.Contains(screen, "retry-uploads → release-2") {
		t.Errorf("scrolling to the end should move the header off screen:\n%s", screen)
	}
}

func TestEscClosesTheModal(t *testing.T) {
	h := newModalHarness(t, "detail_behind.json")

	h.press("enter")
	h.waitForText("Behind main. Update branch available.")
	h.press("esc")
	h.waitForScreen("the modal to close", func(s string) bool {
		return !strings.Contains(s, "Behind main") && strings.Contains(s, "octo-org/sample-repo#12")
	})
}

func TestModalRefreshesWhileOpenAndStopsWhenClosed(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json", "detail_conflict_refreshed.json")
	h.resize(120, 60)

	h.doubleClickCard("octo-org/sample-repo#11")
	h.waitForText("5 passed · 1 failed · 3 running · 1 skipped")

	h.settle(59 * time.Second)
	time.Sleep(50 * time.Millisecond)
	if n := len(detailRequests(h)); n != 1 {
		t.Fatalf("want no detail refresh before the interval, got %d requests", n)
	}
	h.advance(time.Second)
	h.waitForDetailRequests(2)
	h.waitForText("6 passed · 1 failed · 2 running · 1 skipped")
	h.waitForText("Add retry and backoff to the upload worker")

	h.press("esc")
	h.waitForScreen("the modal to close", func(s string) bool { return !strings.Contains(s, "release-2") })
	h.settle(2 * time.Minute)
	time.Sleep(50 * time.Millisecond)
	if n := len(detailRequests(h)); n != 2 {
		t.Errorf("a closed modal kept refreshing: %d detail requests", n)
	}
}

func TestFailedModalRefreshKeepsTheLastDetailMarkedStale(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_behind.json"))
	transport.Reply("PullRequestDetail", githubtest.Response{Err: errors.New("network is unreachable")})
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#12")

	h.press("enter")
	h.waitForText("No checks")
	h.settle(time.Minute)
	screen := h.waitForText("network is unreachable")
	assertContains(t, screen, "stale", "No checks", "Behind main. Update branch available.")
}

func TestDetailQueryAsksForNoDiffsOrFiles(t *testing.T) {
	h := newModalHarness(t, "detail_behind.json")

	h.press("enter")
	reqs := h.waitForDetailRequests(1)
	query := reqs[0].Query
	assertContains(t, query, "node(id: $id)", "mergeStateStatus", "statusCheckRollup", "autoMergeRequest")
	for _, bad := range []string{"files", "diff", "patch"} {
		if strings.Contains(query, bad) {
			t.Errorf("detail query asks for %q", bad)
		}
	}
}
