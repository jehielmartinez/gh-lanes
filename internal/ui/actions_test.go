package ui_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// The actions board holds, newest first: a draft (#21), a ready pull request
// whose branch is behind and can be updated (#22), and one the login may not
// change (#23).
const (
	draftRef  = "octo-org/sample-repo#21"
	readyRef  = "octo-org/sample-repo#22"
	lockedRef = "user-a/other-repo#23"
)

const (
	updateBranchOp   = "UpdatePullRequestBranch"
	markReadyOp      = "MarkPullRequestReadyForReview"
	convertToDraftOp = "ConvertPullRequestToDraft"
)

var mutationOK = map[string]string{
	updateBranchOp:   `{"data":{"updatePullRequestBranch":{"pullRequest":{"id":"PR_node_ready"}}}}`,
	markReadyOp:      `{"data":{"markPullRequestReadyForReview":{"pullRequest":{"id":"PR_node_draft"}}}}`,
	convertToDraftOp: `{"data":{"convertPullRequestToDraft":{"pullRequest":{"id":"PR_node_ready"}}}}`,
}

// newActionsHarness starts the app on the actions board. reply queues extra
// replies before the app starts.
func newActionsHarness(t *testing.T, reply func(*githubtest.Transport)) *harness {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_actions.json"))
	if reply != nil {
		reply(transport)
	}
	h := newHarness(t, transport)
	h.waitForText(lockedRef)
	return h
}

func requestsFor(h *harness, op string) []githubtest.Request {
	var reqs []githubtest.Request
	for _, r := range h.transport.Requests() {
		if r.Operation == op {
			reqs = append(reqs, r)
		}
	}
	return reqs
}

// waitForMutation waits for the one request of op and returns its input.
func (h *harness) waitForMutation(op string) map[string]any {
	h.t.Helper()
	h.waitFor(op, func() bool { return len(requestsFor(h, op)) > 0 })
	reqs := requestsFor(h, op)
	if len(reqs) != 1 {
		h.t.Fatalf("want one %s request, got %d", op, len(reqs))
	}
	input, _ := reqs[0].Variables["input"].(map[string]any)
	return input
}

func assertInput(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mutation input = %v, want %v", got, want)
	}
}

// assertNoMutations checks that no action reached GitHub. Commands run
// asynchronously, so it first lets anything already started arrive.
func assertNoMutations(t *testing.T, h *harness) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	for _, op := range []string{updateBranchOp, markReadyOp, convertToDraftOp} {
		if n := len(requestsFor(h, op)); n > 0 {
			t.Errorf("%s was sent %d times, want none", op, n)
		}
	}
}

func TestUpdateBranchMergesTheBaseInRightAway(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(updateBranchOp, githubtest.Response{Body: []byte(mutationOK[updateBranchOp])})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_branch_updated.json"))
	})
	if !strings.Contains(cardStatus(t, h.screen.plain(), readyRef), "↓ behind") {
		t.Fatalf("#22 should start behind:\n%s", h.screen.plain())
	}

	h.press("j")
	h.press("u")
	assertInput(t, h.waitForMutation(updateBranchOp), map[string]any{
		"pullRequestId": "PR_node_ready",
		"updateMethod":  "MERGE",
	})
	if strings.Contains(h.screen.plain(), "Rebase branch?") {
		t.Errorf("update branch (merge) should not ask first")
	}

	h.waitForText(readyRef + ": branch updated")
	h.waitFor("the pull request to be fetched again", func() bool {
		reqs := requestsFor(h, "PullRequestsByID")
		return len(reqs) == 1 && reflect.DeepEqual(reqs[0].Variables["ids"], []any{"PR_node_ready"})
	})
	h.waitForScreen("the refreshed card", func(s string) bool {
		return strings.Contains(s, readyRef) && !strings.Contains(cardStatus(t, s, readyRef), "behind")
	})
}

func TestRebaseUpdateAsksFirst(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(updateBranchOp, githubtest.Response{Body: []byte(mutationOK[updateBranchOp])})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_branch_updated.json"))
	})

	h.press("j")
	h.press("U")
	screen := h.waitForText("Rebase branch?")
	assertContains(t, screen, "Rebase retry-loop onto main in "+readyRef, "enter confirm", "esc cancel")

	h.press("esc")
	h.waitForScreen("the dialog to close", func(s string) bool { return !strings.Contains(s, "Rebase branch?") })
	assertNoMutations(t, h)

	h.press("U")
	h.waitForText("Rebase branch?")
	h.press("enter")
	assertInput(t, h.waitForMutation(updateBranchOp), map[string]any{
		"pullRequestId": "PR_node_ready",
		"updateMethod":  "REBASE",
	})
	h.waitForText(readyRef + ": branch rebased")
}

func TestDraftKeyMarksADraftReadyRightAway(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(markReadyOp, githubtest.Response{Body: []byte(mutationOK[markReadyOp])})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_marked_ready.json"))
	})

	h.press("d")
	assertInput(t, h.waitForMutation(markReadyOp), map[string]any{"pullRequestId": "PR_node_draft"})
	h.waitForText(draftRef + ": marked ready for review")
	if n := len(requestsFor(h, convertToDraftOp)); n > 0 {
		t.Errorf("a draft should never be converted to draft")
	}
}

func TestDraftKeyConvertsToDraftAfterConfirming(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(convertToDraftOp, githubtest.Response{Body: []byte(mutationOK[convertToDraftOp])})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_converted_to_draft.json"))
	})

	h.press("j")
	h.press("d")
	screen := h.waitForText("Convert to draft?")
	assertContains(t, screen, "Convert "+readyRef+" to a draft?", "dismiss", "review requests")
	assertNoMutations(t, h)

	h.press("enter")
	assertInput(t, h.waitForMutation(convertToDraftOp), map[string]any{"pullRequestId": "PR_node_ready"})
	h.waitForText(readyRef + ": converted to draft")

	// The refreshed copy is a draft, so d now offers ready for review.
	h.press("?")
	h.waitForText("d ready for review")
}

func TestActionsGitHubWouldRefuseAreNotOffered(t *testing.T) {
	h := newActionsHarness(t, nil)

	h.press("?")
	screen := h.waitForText("close help")
	// The draft's branch can't be updated, but it can be marked ready.
	assertContains(t, screen, "d ready for review")
	if strings.Contains(screen, "update branch") || strings.Contains(screen, "rebase branch") {
		t.Errorf("update branch should be hidden for #21:\n%s", screen)
	}

	h.press("j")
	screen = h.waitForText("u update branch")
	assertContains(t, screen, "U rebase branch", "d convert to draft")

	// The login can't change #23.
	h.press("j")
	screen = h.waitForScreen("no actions in the help", func(s string) bool {
		return !strings.Contains(s, "update branch") && !strings.Contains(s, "convert to draft")
	})
	if strings.Contains(screen, "ready for review") {
		t.Errorf("no draft action should be offered for #23:\n%s", screen)
	}
	h.press("u")
	h.press("U")
	h.press("d")
	time.Sleep(50 * time.Millisecond)
	screen = h.screen.plain()
	if strings.Contains(screen, "Rebase branch?") || strings.Contains(screen, "Convert to draft?") {
		t.Errorf("no dialog should open for an action that isn't offered:\n%s", screen)
	}
	assertNoMutations(t, h)
}

func TestARefusedActionShowsGitHubsMessage(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(updateBranchOp, githubtest.Response{Body: []byte(
			`{"data":{"updatePullRequestBranch":null},"errors":[{"type":"UNPROCESSABLE","path":["updatePullRequestBranch"],"message":"Head branch was modified. Review and try the merge again."}]}`,
		)})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_branch_updated.json"))
	})

	h.press("j")
	h.press("u")
	screen := h.waitForText("couldn't update branch")
	assertContains(t, screen, readyRef+": couldn't update branch: Head branch was modified. Review and try the merge again.")
	if strings.Contains(screen, "GraphQL:") {
		t.Errorf("the toast should carry GitHub's message alone:\n%s", screen)
	}
	h.waitFor("the pull request to be fetched again", func() bool { return len(requestsFor(h, "PullRequestsByID")) == 1 })
}

func TestTheToastClearsAfterAFewSeconds(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.Reply(markReadyOp, githubtest.Response{Body: []byte(mutationOK[markReadyOp])})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_marked_ready.json"))
	})

	h.press("d")
	h.waitForText("marked ready for review")
	h.settle(5 * time.Second)
	h.waitForScreen("the toast to clear", func(s string) bool { return !strings.Contains(s, "marked ready for review") })
}

func TestActionsWorkFromTheModal(t *testing.T) {
	h := newActionsHarness(t, func(tr *githubtest.Transport) {
		tr.ReplyFixture(t, "PullRequestDetail", fixture("detail_ready.json"))
		tr.ReplyFixture(t, "PullRequestDetail", fixture("detail_branch_updated.json"))
		tr.Reply(updateBranchOp, githubtest.Response{Body: []byte(mutationOK[updateBranchOp])})
		tr.ReplyFixture(t, "PullRequestsByID", fixture("nodes_branch_updated.json"))
	})

	h.press("j")
	h.press("enter")
	screen := h.waitForText("Behind main. Update branch available.")
	assertContains(t, screen, "u update branch", "d convert to draft")

	h.press("u")
	assertInput(t, h.waitForMutation(updateBranchOp), map[string]any{
		"pullRequestId": "PR_node_ready",
		"updateMethod":  "MERGE",
	})
	h.waitForText(readyRef + ": branch updated")
	// The modal fetches its pull request again and shows what changed.
	screen = h.waitForText("Blocked from merging into main")
	if strings.Contains(screen, "u update branch") {
		t.Errorf("update branch should be gone once the branch is up to date:\n%s", screen)
	}
	if n := len(requestsFor(h, "PullRequestDetail")); n != 2 {
		t.Errorf("want the modal fetched twice, got %d", n)
	}

	h.press("d")
	h.waitForText("Convert to draft?")
	h.press("esc")
	h.waitForScreen("the dialog to close over the modal", func(s string) bool {
		return !strings.Contains(s, "Convert to draft?") && strings.Contains(s, "Blocked from merging into main")
	})
}
