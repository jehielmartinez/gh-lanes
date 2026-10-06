package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// The merge board holds, newest first: a pull request ready to merge in a repo
// that allows merge commits and squash and leaves branches behind (#31), one
// whose checks are still running (#32), one already set to auto-merge (#33),
// one that conflicts with its base (#34), and one ready to merge in a repo
// that deletes merged branches itself (#35), and one blocked by a failed
// check with nothing left for auto-merge to wait for (#36).
const (
	cleanRef     = "octo-org/sample-repo#31"
	pendingRef   = "octo-org/sample-repo#32"
	autoRef      = "octo-org/sample-repo#33"
	conflictRef  = "octo-org/sample-repo#34"
	selfCleanRef = "octo-org/sample-repo#35"
	blockedRef   = "octo-org/sample-repo#36"
)

const (
	mergeOp            = "MergePullRequest"
	deleteRefOp        = "DeleteRef"
	enableAutoMergeOp  = "EnablePullRequestAutoMerge"
	disableAutoMergeOp = "DisablePullRequestAutoMerge"
)

var mergeOps = []string{mergeOp, deleteRefOp, enableAutoMergeOp, disableAutoMergeOp}

func replyOK(tr *githubtest.Transport, op, body string) {
	tr.Reply(op, githubtest.Response{Body: []byte(body)})
}

const (
	mergeOK            = `{"data":{"mergePullRequest":{"pullRequest":{"id":"PR_node_clean"}}}}`
	deleteRefOK        = `{"data":{"deleteRef":{"clientMutationId":null}}}`
	enableAutoMergeOK  = `{"data":{"enablePullRequestAutoMerge":{"pullRequest":{"id":"PR_node_pending"}}}}`
	disableAutoMergeOK = `{"data":{"disablePullRequestAutoMerge":{"pullRequest":{"id":"PR_node_auto"}}}}`
)

func newMergeHarness(t *testing.T, reply func(*githubtest.Transport)) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_merge.json"))
	if reply != nil {
		reply(transport)
	}
	// Tall enough for all six cards in one lane.
	h := newHarness(t, transport, withTermSize(defaultTermWidth, 40))
	h.waitForText(blockedRef)
	return h
}

// assertNoMergeMutations checks that nothing the merge dialog can send reached
// GitHub.
func assertNoMergeMutations(t *testing.T, h *harness, ops ...string) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	if len(ops) == 0 {
		ops = mergeOps
	}
	for _, op := range ops {
		if n := len(requestsFor(h, op)); n > 0 {
			t.Errorf("%s was sent %d times, want none", op, n)
		}
	}
}

func TestMergeDialogListsOnlyTheMethodsTheRepoAllows(t *testing.T) {
	h := newMergeHarness(t, nil)

	h.press("?")
	h.waitForText("M merge…")
	h.press("?")

	h.press("M")
	screen := h.waitForText("Merge " + cleanRef)
	assertContains(t, screen, "Ready to merge into main.", "Create a merge commit", "Squash and merge", "[ ] Delete branch export-button", "space delete branch", "esc close")
	if strings.Contains(screen, "Rebase and merge") {
		t.Errorf("the repo doesn't allow rebase merges:\n%s", screen)
	}

	h.press("esc")
	h.waitForScreen("the dialog to close", func(s string) bool { return !strings.Contains(s, "Create a merge commit") })
	assertNoMergeMutations(t, h)
}

func TestMergeAsksFirstThenMergesWithTheChosenMethod(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		replyOK(tr, mergeOp, mergeOK)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_merged.json"))
	})

	h.press("M")
	h.waitForText("Squash and merge")
	h.press("j")
	h.press("enter")
	screen := h.waitForText("Merge pull request?")
	assertContains(t, screen, "Merge "+cleanRef+" into main (squash and merge)?", "enter confirm", "esc cancel")
	assertNoMergeMutations(t, h)

	h.press("esc")
	h.waitForScreen("the confirmation to close", func(s string) bool { return !strings.Contains(s, "Merge pull request?") })
	assertNoMergeMutations(t, h)

	h.press("M")
	h.waitForText("Squash and merge")
	h.press("j")
	h.press("enter")
	h.waitForText("Merge pull request?")
	h.press("enter")
	assertInput(t, h.waitForMutation(mergeOp), map[string]any{
		"pullRequestId": "PR_node_clean",
		"mergeMethod":   "SQUASH",
	})
	h.waitForText(cleanRef + ": merged")
	h.waitFor("the pull request to be fetched again", func() bool { return len(requestsFor(h, "PullRequestsByID")) == 1 })
	h.waitForScreen("the card to show the merge", func(s string) bool { return strings.Contains(cardStatus(t, s, cleanRef), "Merged") })
	assertNoMergeMutations(t, h, deleteRefOp, enableAutoMergeOp)
}

func TestTheDeleteBranchToggleDeletesABranchTheRepoWouldKeep(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		replyOK(tr, mergeOp, mergeOK)
		replyOK(tr, deleteRefOp, deleteRefOK)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_merged.json"))
	})

	h.press("M")
	h.waitForText("[ ] Delete branch export-button")
	h.press(" ")
	h.waitForText("[x] Delete branch export-button")
	h.press("enter")
	screen := h.waitForText("Merge pull request?")
	assertContains(t, screen, "Merge "+cleanRef+" into main (create a merge", "then delete export-button?")
	h.press("enter")

	assertInput(t, h.waitForMutation(mergeOp), map[string]any{
		"pullRequestId": "PR_node_clean",
		"mergeMethod":   "MERGE",
	})
	assertInput(t, h.waitForMutation(deleteRefOp), map[string]any{"refId": "REF_node_clean"})
	if reqs := h.transport.Requests(); indexOf(reqs, mergeOp) > indexOf(reqs, deleteRefOp) {
		t.Errorf("the branch was deleted before the merge")
	}
	h.waitForText(cleanRef + ": merged")
}

func TestAFailedBranchDeleteStillReportsTheMerge(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		replyOK(tr, mergeOp, mergeOK)
		replyOK(tr, deleteRefOp, `{"data":{"deleteRef":null},"errors":[{"type":"FORBIDDEN","path":["deleteRef"],"message":"Branch is protected."}]}`)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_merged.json"))
	})

	h.press("M")
	h.waitForText("Delete branch export-button")
	h.press(" ")
	h.press("enter")
	h.waitForText("Merge pull request?")
	h.press("enter")
	h.waitForText(cleanRef + ": merged, but couldn't delete export-button: Branch is protected.")
}

func TestARepoThatDeletesMergedBranchesGetsNoDeleteRef(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		replyOK(tr, mergeOp, mergeOK)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_merged.json"))
	})

	for range 4 {
		h.press("j")
	}
	h.press("M")
	screen := h.waitForText("Merge " + selfCleanRef)
	// The toggle starts at the repo's own setting.
	assertContains(t, screen, "[x] Delete branch drop-legacy-flag (the repository deletes merged branches)")
	h.press("enter")
	h.waitForText("Merge pull request?")
	h.press("enter")
	assertInput(t, h.waitForMutation(mergeOp), map[string]any{
		"pullRequestId": "PR_node_selfclean",
		"mergeMethod":   "MERGE",
	})
	h.waitForText(selfCleanRef + ": merged")
	assertNoMergeMutations(t, h, deleteRefOp)
}

func TestPendingChecksOfferAutoMergeInsteadOfAMerge(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		replyOK(tr, enableAutoMergeOp, enableAutoMergeOK)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_auto_enabled.json"))
	})

	h.press("j")
	h.press("?")
	h.waitForText("M auto-merge…")
	h.press("?")
	h.press("M")
	screen := h.waitForText("Auto-merge " + pendingRef)
	assertContains(t, screen, "Squash when ready", "Rebase when ready")
	for _, unwanted := range []string{"Merge commit when ready", "Squash and merge", "Delete branch"} {
		if strings.Contains(screen, unwanted) {
			t.Errorf("auto-merge dialog shows %q:\n%s", unwanted, screen)
		}
	}

	h.press("j")
	h.press("enter")
	assertInput(t, h.waitForMutation(enableAutoMergeOp), map[string]any{
		"pullRequestId": "PR_node_pending",
		"mergeMethod":   "REBASE",
	})
	h.waitForText(pendingRef + ": auto-merge enabled")
	assertNoMergeMutations(t, h, mergeOp)

	h.waitFor("the pull request to be fetched again", func() bool { return len(requestsFor(h, "PullRequestsByID")) == 1 })
	// The refreshed copy has auto-merge on, so M now offers to turn it off.
	h.press("M")
	h.waitForText("Disable auto-merge")
}

func TestEnabledAutoMergeCanBeDisabled(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		replyOK(tr, disableAutoMergeOp, disableAutoMergeOK)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_auto_disabled.json"))
	})

	h.press("j")
	h.press("j")
	h.press("M")
	screen := h.waitForText("Auto-merge " + autoRef)
	assertContains(t, screen, "Squash when ready, enabled by user-a", "Disable auto-merge")

	h.press("enter")
	assertInput(t, h.waitForMutation(disableAutoMergeOp), map[string]any{"pullRequestId": "PR_node_auto"})
	h.waitForText(autoRef + ": auto-merge disabled")
	assertNoMergeMutations(t, h, mergeOp, enableAutoMergeOp)
}

func TestMergeIsNotOfferedWhenGitHubWouldRefuse(t *testing.T) {
	for _, tc := range []struct {
		name string
		ref  string
		down int
	}{
		{"conflicting", conflictRef, 3},
		{"blocked with nothing to wait for", blockedRef, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newMergeHarness(t, nil)
			for range tc.down {
				h.press("j")
			}
			// The help opens after the cursor has moved, so it describes tc.ref.
			h.press("?")
			screen := h.waitForText("close help")
			if strings.Contains(screen, "merge…") {
				t.Errorf("merge should be hidden for %s:\n%s", tc.ref, screen)
			}
			h.press("M")
			time.Sleep(50 * time.Millisecond)
			if strings.Contains(h.screen.plain(), "Merge "+tc.ref) {
				t.Errorf("no merge dialog should open for %s", tc.ref)
			}
			assertNoMergeMutations(t, h)
		})
	}
}

func TestMergeIsNotOfferedForAPullRequestTheLoginCantChange(t *testing.T) {
	h := newActionsHarness(t, nil)

	h.press("j")
	h.press("j")
	h.press("?")
	screen := h.waitForText("close help")
	if strings.Contains(screen, "merge…") {
		t.Errorf("merge should be hidden for %s:\n%s", lockedRef, screen)
	}
	h.press("M")
	assertNoMergeMutations(t, h)
}

func TestTheMergeDialogOpensFromTheModal(t *testing.T) {
	h := newMergeHarness(t, func(tr *githubtest.Transport) {
		tr.ReplyFixture(t, "PullRequestDetail", fixture("detail_merge_clean.json"))
		tr.ReplyFixture(t, "PullRequestDetail", fixture("detail_merge_clean.json"))
		replyOK(tr, mergeOp, mergeOK)
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_merged.json"))
	})

	h.press("enter")
	screen := h.waitForText("Ready to merge into main.")
	assertContains(t, screen, "M merge…")

	h.press("M")
	h.waitForText("Create a merge commit")
	// Keys go to the dialog, not to the modal behind it.
	h.press("j")
	h.press("enter")
	h.waitForText("Merge pull request?")
	h.press("enter")
	assertInput(t, h.waitForMutation(mergeOp), map[string]any{
		"pullRequestId": "PR_node_clean",
		"mergeMethod":   "SQUASH",
	})
	h.waitForText(cleanRef + ": merged")
}

func indexOf(reqs []githubtest.Request, op string) int {
	for i, r := range reqs {
		if r.Operation == op {
			return i
		}
	}
	return -1
}
