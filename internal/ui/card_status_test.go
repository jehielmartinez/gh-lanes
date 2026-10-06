package ui_test

import (
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

func newStatusHarness(t *testing.T) (*harness, string) {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	h := newHarness(t, transport)
	return h, h.waitForText("octo-org/sample-repo#12")
}

// cardStatus returns the status line of the card for ref: its last line,
// under the owner, repository and title.
func cardStatus(t *testing.T, screen, ref string) string {
	t.Helper()
	return cardLine(t, screen, ref, 3)
}

func TestCardShowsChecksReviewDecisionMergeabilityAndAge(t *testing.T) {
	_, screen := newStatusHarness(t)

	for ref, want := range map[string][]string{
		"octo-org/sample-repo#11": {"✓3 ✗1 ●1 approved ⚠ conflict", "3h"},
		"user-a/other-repo#13":    {"●1 changes req. ↓ behind", "20m"},
		"octo-org/sample-repo#12": {"needs review checking…", "2d"},
		"user-a/other-repo#14":    {"✓1 ✗2", "5d"},
	} {
		line := cardStatus(t, screen, ref)
		for _, w := range want {
			if !strings.Contains(line, w) {
				t.Errorf("%s status line = %q, want it to contain %q", ref, line, w)
			}
		}
	}
}

func TestUnknownMergeabilityShowsCheckingNotConflict(t *testing.T) {
	_, screen := newStatusHarness(t)

	line := cardStatus(t, screen, "octo-org/sample-repo#12")
	if !strings.Contains(line, "checking…") {
		t.Errorf("UNKNOWN mergeability should read checking…, got %q", line)
	}
	for _, bad := range []string{"conflict", "⚠", "behind"} {
		if strings.Contains(line, bad) {
			t.Errorf("UNKNOWN mergeability shown as %q: %q", bad, line)
		}
	}
}

func TestCardsSortByMostRecentlyUpdated(t *testing.T) {
	_, screen := newStatusHarness(t)

	order := []string{"other-repo#13", "sample-repo#11", "sample-repo#12", "other-repo#14"}
	last := -1
	for _, ref := range order {
		i := strings.Index(screen, ref)
		if i < last {
			t.Fatalf("cards should be ordered %v:\n%s", order, screen)
		}
		last = i
	}
}

func TestListQueryFetchesStatusFields(t *testing.T) {
	h, _ := newStatusHarness(t)

	query := h.transport.Requests()[0].Query
	for _, field := range []string{
		"state", "isDraft", "baseRefName", "headRefName",
		"mergeable", "mergeStateStatus", "reviewDecision", "autoMergeRequest", "viewerCanUpdate", "viewerCanUpdateBranch",
		"mergeCommitAllowed", "squashMergeAllowed", "rebaseMergeAllowed", "autoMergeAllowed", "deleteBranchOnMerge",
		"statusCheckRollup", "... on CheckRun", "... on StatusContext",
		"comments { totalCount }", "reviews { totalCount }", "latestReviews",
		"rateLimit",
	} {
		if !strings.Contains(query, field) {
			t.Errorf("list query is missing %q", field)
		}
	}
}

func TestChecksArePagedThrough(t *testing.T) {
	h, _ := newStatusHarness(t)

	var paged []githubtest.Request
	for _, r := range h.transport.Requests() {
		if r.Operation == "CheckContexts" {
			paged = append(paged, r)
		}
	}
	if len(paged) != 1 {
		t.Fatalf("want 1 CheckContexts request for the PR with more checks, got %d", len(paged))
	}
	if got := paged[0].Variables["id"]; got != "C_node_paged" {
		t.Errorf("id = %v, want the head commit of the paged PR", got)
	}
	if got := paged[0].Variables["after"]; got != "Y3Vyc29yOjE=" {
		t.Errorf("after = %v, want the first page's end cursor", got)
	}
}
