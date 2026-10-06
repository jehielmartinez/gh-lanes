package ui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

const (
	designDoc   = "https://github.com/octo-org/sample-repo/wiki/Design"
	issue7      = "https://github.com/octo-org/sample-repo/issues/7"
	releaseDoc  = "https://github.com/octo-org/sample-repo/blob/main/docs/release/steps-for-every-supported-platform.md"
	buildLog    = "https://ci.example.com/octo-org/sample-repo/build/9"
	planIssue   = "https://github.com/octo-org/sample-repo/issues/1"
	samePR      = "https://github.com/octo-org/sample-repo/pull/5"
	buildCheck  = "https://github.com/octo-org/sample-repo/actions/runs/3/job/1"
	statusCheck = "https://ci.example.com/octo-org/sample-repo/3"
	linksPR     = "https://github.com/octo-org/sample-repo/pull/21"
	// selectedPR is the card selected when the status board first loads, the
	// most recently updated one.
	selectedPR   = "https://github.com/user-a/other-repo/pull/13"
	selectedPRID = "PR_node_behind"
)

func linksTransport(t *testing.T) *githubtest.Transport {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	transport.ReplyFixture(t, "PullRequestDetail", fixture("detail_links.json"))
	return transport
}

// newLinksHarness opens the detail modal on the links fixture in a terminal
// of the given size and waits for its description.
func newLinksHarness(t *testing.T, width, height int, opts ...harnessOption) *harness {
	t.Helper()
	h := newHarness(t, linksTransport(t), opts...)
	h.waitForText("octo-org/sample-repo#12")
	h.resize(width, height)
	h.press("enter")
	h.waitForText("Description")
	return h
}

// clickWithin clicks offset cells into the first place text appears on
// screen.
func (h *harness) clickWithin(text string, offset int) {
	h.t.Helper()
	x, y, ok := locate(h.waitForText(text), text)
	if !ok {
		h.t.Fatalf("%q is not on screen", text)
	}
	h.click(x+offset, y)
}

func TestClickingAMarkdownLinkOpensItsURL(t *testing.T) {
	h := newLinksHarness(t, 120, 120)

	h.clickText("design doc")
	h.waitForOpened(designDoc)
}

func TestClickingABareURLOpensIt(t *testing.T) {
	h := newLinksHarness(t, 120, 120)

	h.clickWithin("issues/7", 3)
	h.waitForOpened(issue7)
}

func TestClickingTheWrappedPartOfALinkOpensTheWholeURL(t *testing.T) {
	h := newLinksHarness(t, 80, 120)

	screen := h.waitForText("supported-platform.md")
	_, first, _ := locate(screen, "https://github.com/octo-org/sample-repo/blob")
	_, second, _ := locate(screen, "supported-platform.md")
	if second != first+1 {
		t.Fatalf("want the release notes link wrapped onto a second line:\n%s", screen)
	}
	h.clickText("supported-platform.md")
	h.waitForOpened(releaseDoc)
}

func TestClickingTextThatIsNotALinkOpensNothing(t *testing.T) {
	h := newLinksHarness(t, 120, 120)

	h.clickText("Release notes:")
	h.clickText("Document the release steps")
	h.clickText("design doc")
	h.waitForOpened(designDoc)
}

func TestLinksStayClickableAfterScrolling(t *testing.T) {
	h := newLinksHarness(t, 120, 30)

	h.press("G")
	h.waitForScreen("the header scrolled away", func(s string) bool {
		return !strings.Contains(s, "Document the release steps") && strings.Contains(s, "the plan")
	})
	h.clickText("the plan")
	h.waitForOpened(planIssue)
}

func TestClickingACheckOpensItsDetailsPage(t *testing.T) {
	h := newLinksHarness(t, 120, 120)

	raw := h.screen.content()
	if !underlined(raw, "build") {
		t.Errorf("the check name is not underlined:\n%q", raw)
	}
	h.clickWithin("✓ build", 2)
	h.clickWithin("✓ ci/external", 4)
	h.waitForOpened(buildCheck, statusCheck)
}

func TestLinksOtherThanHTTPAreNeitherUnderlinedNorOpened(t *testing.T) {
	h := newLinksHarness(t, 120, 120)

	raw := h.screen.content()
	for _, text := range []string{"this one", "that one"} {
		if underlined(raw, text) {
			t.Errorf("%q is underlined as a link", text)
		}
		h.clickText(text)
	}
	h.clickText("design doc")
	h.waitForOpened(designDoc)
}

func TestLinkPickerListsEveryURLWithItsSourceAndAuthor(t *testing.T) {
	h := newLinksHarness(t, 160, 60)

	h.press("o")
	screen := h.waitForText("Links")
	assertRowOrder(t, screen,
		buildCheck+" check build",
		statusCheck+" check ci/external",
		designDoc+" description · user-a",
		issue7+" description · user-a",
		releaseDoc+" description · user-a",
		buildLog+" comment · user-b",
		samePR+" thread docs/release.md:4 · user-b",
		planIssue+" review · user-c",
	)
	for _, unsafe := range []string{"javascript:", "file:"} {
		if strings.Contains(screen, unsafe) {
			t.Errorf("the picker lists a %s link:\n%s", unsafe, screen)
		}
	}

	for range 5 {
		h.press("j")
	}
	h.press("enter")
	h.waitForOpened(buildLog)
	h.waitForScreen("the picker to close", func(s string) bool { return !strings.Contains(s, "comment · user-b") })
}

func TestLinkPickerClosesOnEscWithoutOpening(t *testing.T) {
	h := newLinksHarness(t, 160, 60)

	h.press("o")
	h.waitForText("description · user-a")
	h.press("esc")
	h.waitForScreen("the picker to close", func(s string) bool { return !strings.Contains(s, "description · user-a") })
	h.press("O")
	h.waitForOpened(linksPR)
}

func TestLinkPickerOnTheBoardLoadsThePullRequestFirst(t *testing.T) {
	h := newHarness(t, linksTransport(t), withTermSize(160, 60))
	h.waitForText("octo-org/sample-repo#12")

	h.press("o")
	h.waitForText("description · user-a")
	reqs := detailRequests(h)
	if len(reqs) != 1 || reqs[0].Variables["id"] != selectedPRID {
		t.Fatalf("want one detail request for %s, got %+v", selectedPRID, reqs)
	}
	h.press("enter")
	h.waitForOpened(buildCheck)
}

func TestLinkPickerOnTheBoardShowsWhyItCouldNotLoad(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	transport.Reply("PullRequestDetail", githubtest.Response{Err: errors.New("connection reset")})
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#12")

	h.press("o")
	h.waitForText("Couldn't load links:")
	h.waitForText("connection reset")
}

func TestShiftOOpensThePullRequestInTheBrowser(t *testing.T) {
	h := newHarness(t, linksTransport(t))
	h.waitForText("octo-org/sample-repo#12")

	h.press("O")
	h.waitForOpened(selectedPR)

	h.press("enter")
	h.waitForText("Document the release steps")
	h.press("O")
	h.waitForOpened(selectedPR, linksPR)
}

func TestAFailedOpenIsShownInTheStatusBar(t *testing.T) {
	h := newHarness(t, linksTransport(t), withOpenError(errors.New("no browser found")))
	h.waitForText("octo-org/sample-repo#12")

	h.press("O")
	h.waitForText("Couldn't open link: no browser found")
}

// assertRowOrder checks that each string appears on a screen line of its own,
// each below the one before.
func assertRowOrder(t *testing.T, screen string, rows ...string) {
	t.Helper()
	last := -1
	for _, r := range rows {
		_, y, ok := locate(screen, r)
		if !ok || y <= last {
			t.Fatalf("want %q on a line below the one before:\n%s", r, screen)
		}
		last = y
	}
}
