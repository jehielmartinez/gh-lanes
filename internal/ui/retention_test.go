package ui_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

const (
	mergedPR = "PR_node_merged" // octo-org/sample-repo#20, merged
	closedPR = "PR_node_closed" // user-a/other-repo#21, closed without merging
)

// startWithState starts the app on a config directory whose state file holds
// state, and waits until the lanes it describes are on screen.
func startWithState(t *testing.T, transport *githubtest.Transport, state, wait string) *harness {
	t.Helper()
	dir := t.TempDir()
	seedFile(t, dir, "state.json", state)
	h := newHarness(t, transport, withConfigDir(dir))
	h.waitForText(wait)
	return h
}

// refreshUntil advances the clock a second at a time until the app has run n
// open searches, however long each refresh took to land.
func (h *harness) refreshUntil(n int) {
	h.t.Helper()
	for range 10 * 60 {
		if searches(h) >= n {
			return
		}
		h.advance(time.Second)
	}
	h.t.Fatalf("no refresh %d after ten minutes", n)
}

func byIDRequests(h *harness) [][]string {
	var batches [][]string
	for _, r := range h.transport.Requests() {
		if r.Operation != "PullRequestsByID" {
			continue
		}
		var ids []string
		raw, _ := r.Variables["ids"].([]any)
		for _, id := range raw {
			s, _ := id.(string)
			ids = append(ids, s)
		}
		batches = append(batches, ids)
	}
	return batches
}

// waitForArchived waits until the state file's archived list is exactly want.
func (h *harness) waitForArchived(want []archivedJSON) {
	h.t.Helper()
	h.waitFor("archived list", func() bool {
		s, ok := readState(h.configDir)
		return ok && s.Version == 1 && reflect.DeepEqual(nilIfEmpty(s.Archived), nilIfEmpty(want))
	})
}

func nilIfEmpty(a []archivedJSON) []archivedJSON {
	if len(a) == 0 {
		return nil
	}
	return a
}

func TestTaggedPRsMissingFromTheSearchAreFetchedByID(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	h := startWithState(t, transport,
		`{"version":1,"assignments":{"PR_node_older":"testing","PR_node_merged":"demo","PR_node_closed":"done"}}`,
		"Done 1")

	header := headerLine(h.screen.plain())
	for _, want := range []string{"Untagged 2", "Testing 1", "Demo 1", "Done 1"} {
		if !strings.Contains(header, want) {
			t.Errorf("header is missing %q:\n%s", want, header)
		}
	}
	if got, want := byIDRequests(h), [][]string{{closedPR, mergedPR}}; !reflect.DeepEqual(got, want) {
		t.Errorf("fetched by ID %v, want %v: only tagged PRs missing from the search", got, want)
	}
	screen := h.screen.plain()
	if line := cardStatus(t, screen, "octo-org/sample-repo#20"); !strings.Contains(line, "Merged") {
		t.Errorf("merged card should carry a Merged badge: %q", line)
	}
	if line := cardStatus(t, screen, "user-a/other-repo#21"); !strings.Contains(line, "Closed") {
		t.Errorf("closed card should carry a Closed badge: %q", line)
	}
}

func TestNothingIsFetchedByIDWhileEveryTaggedPRIsOpen(t *testing.T) {
	h := startWithState(t, boardTransport(t), `{"version":1,"assignments":{"PR_node_older":"testing"}}`, "Testing 1")

	h.refreshUntil(2)
	h.settle(time.Second)
	if got := byIDRequests(h); len(got) != 0 {
		t.Errorf("fetched %v by ID, want nothing", got)
	}
}

func TestUntaggedPRsDropOffAndTaggedOnesStayOnceNoLongerOpen(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board_older_only.json"))
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_newer_merged.json"))
	h := startWithState(t, transport, `{"version":1,"assignments":{"PR_node_newer":"in-progress"}}`, "In Progress 1")
	if line := cardStatus(t, h.screen.plain(), "user-a/other-repo#42"); strings.Contains(line, "Merged") {
		t.Fatalf("an open PR should carry no state badge: %q", line)
	}

	h.settle(60 * time.Second)
	screen := h.waitForText("Merged")
	header := headerLine(screen)
	if !strings.Contains(header, "Untagged 1") || !strings.Contains(header, "In Progress 1") {
		t.Errorf("the untagged PR that left the search should drop off, the tagged one stay:\n%s", header)
	}
	if strings.Contains(screen, "a-very-long-sample-repository-name") {
		t.Errorf("untagged PR no longer open is still shown:\n%s", screen)
	}
	if line := cardStatus(t, screen, "user-a/other-repo#42"); !strings.Contains(line, "Merged") {
		t.Errorf("tagged PR should stay with a Merged badge: %q", line)
	}
	if got, want := byIDRequests(h), [][]string{{"PR_node_newer"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("fetched by ID %v, want %v", got, want)
	}
}

func TestAPRWhoseTagWasDeletedIsUntaggedAndNotKept(t *testing.T) {
	h := startWithState(t, boardTransport(t),
		`{"version":1,"assignments":{"PR_node_older":"tag-gone","PR_node_merged":"tag-gone"}}`,
		"Untagged 3")

	h.refreshUntil(2)
	h.settle(time.Second)
	if got := byIDRequests(h); len(got) != 0 {
		t.Errorf("fetched %v by ID; a PR whose tag is gone is untagged, so not tracked", got)
	}
}

func TestTrackedPRsThatNoLongerResolveAreSkipped(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_not_found.json"))
	h := startWithState(t, transport,
		`{"version":1,"assignments":{"PR_node_merged":"demo","PR_node_gone":"review"}}`,
		"Demo 1")

	screen := h.screen.plain()
	if strings.Contains(screen, "Couldn't") {
		t.Errorf("a PR that no longer resolves must not fail the refresh:\n%s", screen)
	}
	if header := headerLine(screen); !strings.Contains(header, "Review 0") {
		t.Errorf("the unresolvable PR should not be shown:\n%s", header)
	}
}

func TestAFailedFetchByIDKeepsTheLastBoard(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	transport.Reply("PullRequestsByID", githubtest.Response{Status: 502, Body: []byte(`{"message":"Bad Gateway"}`)})
	h := startWithState(t, transport, `{"version":1,"assignments":{"PR_node_merged":"demo"}}`, "Demo 1")

	h.settle(60 * time.Second)
	screen := h.waitForText("Couldn't refresh")
	if !strings.Contains(screen, "stale") || !strings.Contains(headerLine(screen), "Demo 1") {
		t.Errorf("the last good board should stay, marked stale:\n%s", screen)
	}
}

func TestArchiveTakesTheSelectedCardOffTheBoard(t *testing.T) {
	h := startBoard(t)

	h.press("x")
	h.waitForArchived([]archivedJSON{{ID: newestPR, Open: true}})
	screen := h.waitForText("Untagged 2")
	if strings.Contains(screen, "user-a/other-repo#42") {
		t.Errorf("archived card is still on the board:\n%s", screen)
	}
}

func TestArchiveFromATagLaneRemovesTheAssignment(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	h := startWithState(t, transport,
		`{"version":1,"assignments":{"PR_node_older":"review","PR_node_closed":"done"}}`,
		"Done 1")

	for range 5 {
		h.press("l")
	}
	h.press("x")
	h.waitForArchived([]archivedJSON{{ID: closedPR, Open: false}})
	h.waitForAssignments(map[string]string{olderPR: "review"})
	h.waitForText("Done 0")

	h.refreshUntil(2)
	h.waitForRequests(3)
	h.settle(time.Second)
	if got, want := byIDRequests(h), [][]string{{closedPR}}; !reflect.DeepEqual(got, want) {
		t.Errorf("fetched by ID %v, want %v: an archived PR is no longer tracked", got, want)
	}
	if header := headerLine(h.screen.plain()); !strings.Contains(header, "Done 0") {
		t.Errorf("archived PR came back:\n%s", header)
	}
}

func TestArchivedPRsDoNotComeBackOnRefresh(t *testing.T) {
	h := startBoard(t)
	h.press("x")
	h.waitForArchived([]archivedJSON{{ID: newestPR, Open: true}})

	h.refreshUntil(2)
	h.settle(time.Second)
	screen := h.waitForText("Untagged 2")
	if strings.Contains(screen, "user-a/other-repo#42") {
		t.Errorf("archived card came back on refresh:\n%s", screen)
	}
	h.waitForArchived([]archivedJSON{{ID: newestPR, Open: true}})
}

func TestAnArchivedPRSeenClosedReturnsToUntaggedWhenReopened(t *testing.T) {
	h := startWithState(t, boardTransport(t),
		`{"version":1,"assignments":{},"archived":[{"id":"PR_node_older","open":false}]}`,
		"Untagged 3")

	h.waitForArchived(nil)
}

func TestAPRArchivedWhileOpenReturnsOnlyOnceClosedAndReopened(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board_older_only.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport)
	h.waitForScreen("the board with its tag lanes", func(s string) bool {
		return strings.Contains(s, "user-a/other-repo#42") && hasTagLanes(s)
	})
	h.press("x")
	h.waitForArchived([]archivedJSON{{ID: newestPR, Open: true}})

	h.refreshUntil(2)
	h.settle(time.Second)
	if strings.Contains(h.screen.plain(), "user-a/other-repo#42") {
		t.Fatalf("still open, so still archived:\n%s", h.screen.plain())
	}

	h.refreshUntil(3)
	h.waitForArchived([]archivedJSON{{ID: newestPR, Open: false}})

	h.refreshUntil(4)
	h.waitForText("user-a/other-repo#42")
	h.waitForText("Untagged 3")
	h.waitForArchived(nil)
}
