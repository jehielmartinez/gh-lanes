package ui_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// startArchived starts the board on a state file holding archived, with the
// tracked fixture queued for the archived tab's fetch by ID.
func startArchived(t *testing.T, archived string) *harness {
	t.Helper()
	transport := boardTransport(t)
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	return startWithState(t, transport, `{"version":1,"assignments":{},"archived":`+archived+`}`, "Untagged 3")
}

func TestArchivedTabListsTheArchivedPRs(t *testing.T) {
	h := startArchived(t, `[{"id":"PR_node_merged","open":false,"tag":"testing"},{"id":"PR_node_closed","open":false}]`)
	h.waitForText("Archived 2")
	h.settle(time.Second)
	if got := byIDRequests(h); len(got) != 0 {
		t.Fatalf("archived PRs fetched before their tab was opened: %v", got)
	}

	h.press("tab")
	h.press("tab")
	screen := h.waitForText("Ship the upload worker")
	assertContains(t, screen, "Try a different cache", "x unarchive")
	if got, want := byIDRequests(h), [][]string{{mergedPR, closedPR}}; !reflect.DeepEqual(got, want) {
		t.Errorf("fetched by ID %v, want %v", got, want)
	}
}

func TestUnarchiveRestoresTheLaneItWasArchivedFrom(t *testing.T) {
	h := startArchived(t, `[{"id":"PR_node_merged","open":false,"tag":"testing"}]`)

	h.clickText("Archived 1")
	h.waitForText("Ship the upload worker")
	h.press("x")
	h.waitForText("restored to Testing")
	h.waitForArchived(nil)
	h.waitForAssignments(map[string]string{mergedPR: "testing"})
	h.waitForText("Archived 0")

	h.press("shift+tab")
	h.press("shift+tab")
	h.waitForScreen("the merged PR back in Testing", func(s string) bool {
		return strings.Contains(headerLine(s), "Testing 1") && strings.Contains(s, "Ship the upload worker")
	})
}

func TestUnarchivingAClosedUntaggedPRSaysItStaysOff(t *testing.T) {
	h := startArchived(t, `[{"id":"PR_node_closed","open":false}]`)

	h.clickText("Archived 1")
	h.waitForText("Try a different cache")
	h.press("x")
	h.waitForText("stays off the board")
	h.waitForArchived(nil)
	h.waitForText("Nothing archived.")
}

func TestArchivingRecordsTheTagAndShowsOnTheArchivedTab(t *testing.T) {
	transport := boardTransport(t)
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	transport.ReplyFixture(t, "PullRequestsByID", fixture("nodes_tracked.json"))
	h := startWithState(t, transport, `{"version":1,"assignments":{"PR_node_closed":"done"}}`, "Done 1")

	for range 5 {
		h.press("l")
	}
	h.press("x")
	h.waitForArchived([]archivedJSON{{ID: closedPR, Open: false, Tag: "done"}})
	h.clickText("Archived 1")
	h.waitForText("Try a different cache")
}

func TestEmptyArchiveSaysHowToArchive(t *testing.T) {
	h := startBoard(t)

	h.clickText("Archived 0")
	h.waitForText("Nothing archived. Press x on a board card to archive it.")
	if got := byIDRequests(h); len(got) != 0 {
		t.Errorf("nothing is archived, yet fetched by ID: %v", got)
	}
}

func TestAFailedArchivedFetchSaysWhy(t *testing.T) {
	transport := boardTransport(t)
	transport.Reply("PullRequestsByID", githubtest.Response{Err: errors.New("dial tcp: network is unreachable")})
	h := startWithState(t, transport, `{"version":1,"assignments":{},"archived":[{"id":"PR_node_closed","open":false}]}`, "Untagged 3")

	h.clickText("Archived 1")
	h.waitForText("Couldn't load archived pull requests:")
}
