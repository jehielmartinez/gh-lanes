package ui_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// statusRefs are the cards of search_status.json by node ID.
var statusRefs = map[string]string{
	"PR_node_conflict": "octo-org/sample-repo#11",
	"PR_node_checking": "octo-org/sample-repo#12",
	"PR_node_behind":   "user-a/other-repo#13",
	"PR_node_paged":    "user-a/other-repo#14",
}

// cardMarker returns what the card whose first line holds ref shows after
// its reference: "dot", "new" or "".
func cardMarker(t *testing.T, screen, ref string) string {
	t.Helper()
	for _, l := range strings.Split(screen, "\n") {
		at := strings.Index(l, ref)
		if at < 0 {
			continue
		}
		rest := l[at+len(ref):]
		if end := strings.IndexAny(rest, "│┃"); end >= 0 {
			rest = rest[:end]
		}
		switch strings.TrimSpace(rest) {
		case "●":
			return "dot"
		case "new":
			return "new"
		case "":
			return ""
		}
		t.Fatalf("unexpected text after %s: %q", ref, rest)
	}
	t.Fatalf("no card for %s:\n%s", ref, screen)
	return ""
}

func assertMarkers(t *testing.T, screen string, refs map[string]string, want map[string]string) {
	t.Helper()
	for id, ref := range refs {
		if got := cardMarker(t, screen, ref); got != want[id] {
			t.Errorf("%s marker = %q, want %q:\n%s", ref, got, want[id], screen)
		}
	}
}

type snapshotJSON struct {
	SeenAt         time.Time `json:"seen_at"`
	Checks         string    `json:"checks"`
	Mergeable      string    `json:"mergeable"`
	ReviewDecision string    `json:"review_decision"`
	Comments       int       `json:"comments"`
	Reviews        int       `json:"reviews"`
	Draft          bool      `json:"draft"`
	State          string    `json:"state"`
}

func readSnapshots(dir string) (map[string]snapshotJSON, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, false
	}
	var s struct {
		Snapshots map[string]snapshotJSON `json:"snapshots"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s.Snapshots, s.Snapshots != nil
}

// waitForSnapshots waits until the state file holds a snapshot for every ID.
func (h *harness) waitForSnapshots(ids ...string) map[string]snapshotJSON {
	h.t.Helper()
	var snaps map[string]snapshotJSON
	h.waitFor("snapshots of "+strings.Join(ids, ", "), func() bool {
		var ok bool
		snaps, ok = readSnapshots(h.configDir)
		for _, id := range ids {
			if _, has := snaps[id]; !has {
				return false
			}
		}
		return ok
	})
	return snaps
}

// editSnapshot sets one field of a pull request's snapshot in the state file.
func editSnapshot(t *testing.T, dir, id, field string, value any) {
	t.Helper()
	path := filepath.Join(dir, "state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	s["snapshots"].(map[string]any)[id].(map[string]any)[field] = value
	raw, err = json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func statusTransport(t *testing.T) *githubtest.Transport {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	return transport
}

// baselineStatusBoard runs the app once on the status board, so its pull
// requests are recorded as seen, and returns the config directory.
func baselineStatusBoard(t *testing.T) string {
	t.Helper()
	h := newHarness(t, statusTransport(t))
	h.waitForText("octo-org/sample-repo#12")
	h.waitForSnapshots("PR_node_conflict", "PR_node_checking", "PR_node_behind", "PR_node_paged")
	h.press("q")
	h.waitFinished()
	return h.configDir
}

func TestFirstRunMarksNothingAndRecordsWhatWasOnTheBoard(t *testing.T) {
	h := newHarness(t, statusTransport(t))
	screen := h.waitForText("octo-org/sample-repo#12")

	assertMarkers(t, screen, statusRefs, nil)
	if strings.Contains(screen, "changed") || strings.Contains(screen, "new PR") {
		t.Errorf("the first run should report no activity:\n%s", screen)
	}
	snaps := h.waitForSnapshots("PR_node_conflict", "PR_node_checking", "PR_node_behind", "PR_node_paged")
	want := snapshotJSON{
		SeenAt:         time.Date(2026, 3, 5, 12, 0, 0, 0, fixedZone),
		Checks:         "FAILED",
		Mergeable:      "CONFLICTING",
		ReviewDecision: "APPROVED",
		Comments:       2,
		Reviews:        1,
		State:          "OPEN",
	}
	got := snaps["PR_node_conflict"]
	if !got.SeenAt.Equal(want.SeenAt) {
		t.Errorf("seen_at = %v, want %v", got.SeenAt, want.SeenAt)
	}
	got.SeenAt = want.SeenAt
	if got != want {
		t.Errorf("snapshot = %+v, want %+v", got, want)
	}
	if s, _ := readState(h.configDir); s.Version != 1 {
		t.Errorf("state version = %d, want 1", s.Version)
	}
}

func TestCardShowsADotWhenItsPullRequestChangedSinceLastSeen(t *testing.T) {
	for _, tc := range []struct {
		field string
		value any
	}{
		{"comments", 1},
		{"reviews", 0},
		{"checks", "PASSED"},
		{"mergeable", "MERGEABLE"},
		{"review_decision", "REVIEW_REQUIRED"},
		{"draft", true},
		{"state", "CLOSED"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			dir := baselineStatusBoard(t)
			editSnapshot(t, dir, "PR_node_conflict", tc.field, tc.value)

			h := newHarness(t, statusTransport(t), withConfigDir(dir))
			screen := h.waitForText("1 PR changed")
			assertMarkers(t, screen, statusRefs, map[string]string{"PR_node_conflict": "dot"})
		})
	}
}

func TestStatusBarCountsChangedPullRequests(t *testing.T) {
	dir := baselineStatusBoard(t)
	editSnapshot(t, dir, "PR_node_conflict", "comments", 0)
	editSnapshot(t, dir, "PR_node_paged", "checks", "PENDING")

	h := newHarness(t, statusTransport(t), withConfigDir(dir))
	screen := h.waitForText("2 PRs changed")
	assertMarkers(t, screen, statusRefs, map[string]string{"PR_node_conflict": "dot", "PR_node_paged": "dot"})
}

func TestUnknownMergeabilityIsNeverAChange(t *testing.T) {
	dir := baselineStatusBoard(t)
	// #12 is UNKNOWN now; #13 is MERGEABLE now but was UNKNOWN when seen.
	editSnapshot(t, dir, "PR_node_checking", "mergeable", "CONFLICTING")
	editSnapshot(t, dir, "PR_node_behind", "mergeable", "UNKNOWN")

	h := newHarness(t, statusTransport(t), withConfigDir(dir))
	screen := h.waitForText("octo-org/sample-repo#12")
	assertMarkers(t, screen, statusRefs, nil)
	if strings.Contains(screen, "changed") {
		t.Errorf("checking mergeability must not count as a change:\n%s", screen)
	}
}

func TestOpeningAPullRequestClearsItsDotAndUpdatesItsSnapshot(t *testing.T) {
	dir := baselineStatusBoard(t)
	editSnapshot(t, dir, "PR_node_conflict", "comments", 0)

	transport := statusTransport(t)
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_conflict.json"))
	h := newHarness(t, transport, withConfigDir(dir))
	h.waitForText("1 PR changed")
	h.settle(5 * time.Second)

	h.doubleClickCard("octo-org/sample-repo#11")
	h.waitForText("retry-uploads → release-2")
	h.press("esc")
	screen := h.waitForScreen("the modal to close", func(s string) bool { return !strings.Contains(s, "retry-uploads") })

	assertMarkers(t, screen, statusRefs, nil)
	if strings.Contains(screen, "changed") {
		t.Errorf("opening the only changed pull request should clear the count:\n%s", screen)
	}
	var snap snapshotJSON
	h.waitFor("the opened pull request's snapshot to update", func() bool {
		snaps, _ := readSnapshots(h.configDir)
		snap = snaps["PR_node_conflict"]
		return snap.Comments == 2
	})
	if want := time.Date(2026, 3, 5, 12, 0, 5, 0, fixedZone); !snap.SeenAt.Equal(want) {
		t.Errorf("seen_at = %v, want %v", snap.SeenAt, want)
	}
}

func TestPullRequestsAppearingAfterTheFirstRunAreNew(t *testing.T) {
	first := startBoard(t)
	first.waitForSnapshots(newestPR, olderPR, oldestPR)
	first.press("q")
	first.waitFinished()

	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board_added.json"))
	h := newHarness(t, transport, withConfigDir(first.configDir))
	screen := h.waitForText("1 new PR")
	refs := map[string]string{
		"PR_node_added": "octo-org/sample-repo#8",
		newestPR:        "user-a/other-repo#42",
		olderPR:         "octo-org/sample-repo#7",
	}
	assertMarkers(t, screen, refs, map[string]string{"PR_node_added": "new"})

	h.press("enter")
	h.waitForDetailRequests(1)
	h.waitForText("Couldn't load this pull request")
	h.press("esc")
	screen = h.waitForScreen("the modal to close", func(s string) bool { return !strings.Contains(s, "Couldn't load this pull request") })
	if strings.Contains(screen, "new PR") {
		t.Errorf("opening the only new pull request should clear the count:\n%s", screen)
	}
	assertMarkers(t, screen, refs, nil)
	h.waitForSnapshots("PR_node_added")
}

func TestFirstRunDoesNotMarkAnythingNew(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board_added.json"))
	h := newHarness(t, transport)
	screen := h.waitForText("octo-org/sample-repo#7")
	assertMarkers(t, screen, map[string]string{olderPR: "octo-org/sample-repo#7", newestPR: "user-a/other-repo#42"}, nil)
	if strings.Contains(screen, "new PR") {
		t.Errorf("nothing is new on the first run:\n%s", screen)
	}

	h.press("r")
	screen = h.waitForText("1 new PR")
	if got := cardMarker(t, screen, "octo-org/sample-repo#8"); got != "new" {
		t.Errorf("a pull request appearing after the first load should be new, got %q:\n%s", got, screen)
	}
}

func TestOpeningWhileMergeabilityIsCheckingKeepsTheLastKnownValue(t *testing.T) {
	dir := baselineStatusBoard(t)
	editSnapshot(t, dir, "PR_node_checking", "mergeable", "CONFLICTING")

	transport := statusTransport(t)
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_checking.json"))
	h := newHarness(t, transport, withConfigDir(dir))
	h.waitForText("octo-org/sample-repo#12")
	h.settle(5 * time.Second)

	h.doubleClickCard("octo-org/sample-repo#12")
	h.waitForDetailRequests(1)
	var snap snapshotJSON
	h.waitFor("the opened pull request's snapshot to update", func() bool {
		snaps, _ := readSnapshots(h.configDir)
		snap = snaps["PR_node_checking"]
		return snap.SeenAt.Equal(time.Date(2026, 3, 5, 12, 0, 5, 0, fixedZone))
	})
	if snap.Mergeable != "CONFLICTING" {
		t.Errorf("mergeable = %q; seeing it while GitHub checks must keep the last known value", snap.Mergeable)
	}
}

func TestArchivedPullRequestsAreNotCounted(t *testing.T) {
	first := startBoard(t)
	first.waitForSnapshots(newestPR, olderPR, oldestPR)
	first.press("q")
	first.waitFinished()
	path := filepath.Join(first.configDir, "state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	s["archived"] = []map[string]any{{"id": "PR_node_added", "open": true}}
	raw, _ = json.Marshal(s)
	seedFile(t, first.configDir, "state.json", string(raw))

	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board_added.json"))
	h := newHarness(t, transport, withConfigDir(first.configDir))
	screen := h.waitForText("octo-org/sample-repo#7")
	if strings.Contains(screen, "octo-org/sample-repo#8") || strings.Contains(screen, "new PR") {
		t.Errorf("an archived pull request is off the board and out of the count:\n%s", screen)
	}
}
