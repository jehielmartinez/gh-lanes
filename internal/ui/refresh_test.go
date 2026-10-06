package ui_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

// spinnerFrames are the frames of the status bar's refresh spinner.
var spinnerFrames = strings.ReplaceAll(strings.Join(spinner.Dot.Frames, ""), " ", "")

func searches(h *harness) int {
	n := 0
	for _, r := range h.transport.Requests() {
		if r.Operation == "SearchPullRequests" {
			n++
		}
	}
	return n
}

// assertNoRefreshYet gives a wrongly started refresh time to reach the
// transport before checking that none did.
func assertNoRefreshYet(t *testing.T, h *harness) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	if n := searches(h); n != 1 {
		t.Fatalf("want no refresh yet, got %d searches", n)
	}
}

func heldReply(t *testing.T, path string) (githubtest.Response, chan struct{}) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	return githubtest.Response{Body: body, Release: release}, release
}

func TestBoardRefreshesEveryMinuteByDefault(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")

	h.settle(59 * time.Second)
	assertNoRefreshYet(t, h)

	h.advance(time.Second)
	screen := h.waitForText("octo-org/sample-repo#12")
	if strings.Contains(screen, "octo-org/sample-repo#7") {
		t.Errorf("refreshed board should replace the old one:\n%s", screen)
	}
}

func TestRefreshIntervalComesFromConfig(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport, withConfig("version: 1\nrefresh_interval: 2m\n"))
	h.waitForText("octo-org/sample-repo#7")

	h.advance(time.Minute)
	h.advance(59 * time.Second)
	assertNoRefreshYet(t, h)

	h.advance(time.Second)
	h.waitForRequests(2)
}

func TestInvalidRefreshIntervalIsReportedAndDefaulted(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport, withConfig("version: 1\nrefresh_interval: often\n"))

	h.waitForText(`refresh_interval "often"`)
	h.advance(time.Minute)
	h.waitForRequests(2)
}

func TestRefreshKeyRefreshesNowWithSpinner(t *testing.T) {
	held, release := heldReply(t, fixture("search_board.json"))
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	transport.Reply("SearchPullRequests", held)
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")
	h.settle(5 * time.Second)
	h.waitForText("updated 5s ago")

	h.press("r")
	h.waitForRequests(2)
	h.waitForScreen("spinner beside the updated time", func(s string) bool {
		i := strings.Index(s, " updated 5s ago")
		return i > 0 && strings.ContainsAny(s[max(i-4, 0):i], spinnerFrames)
	})

	close(release)
	screen := h.waitForText("updated 0s ago")
	if strings.ContainsAny(screen, spinnerFrames) {
		t.Errorf("spinner should stop once the refresh lands:\n%s", screen)
	}
}

func TestUIStaysResponsiveDuringRefresh(t *testing.T) {
	held, release := heldReply(t, fixture("search_board.json"))
	defer close(release)
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	transport.Reply("SearchPullRequests", held)
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")

	h.press("r")
	h.waitForRequests(2)
	h.press("q")
	h.waitFinished()
}

func TestFailedRefreshKeepsLastGoodBoardMarkedStale(t *testing.T) {
	for name, reply := range map[string]struct {
		resp githubtest.Response
		want string
	}{
		"server error":  {githubtest.Response{Status: 502, Body: []byte(`{"message":"Server Error"}`)}, "HTTP 502"},
		"offline":       {githubtest.Response{Err: errors.New("dial tcp: lookup api.github.com: no such host")}, "no such host"},
		"expired login": {githubtest.Response{Status: 401, Body: []byte(`{"message":"Bad credentials"}`)}, "run `gh auth login`"},
	} {
		t.Run(name, func(t *testing.T) {
			transport := githubtest.New()
			transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
			transport.Reply("SearchPullRequests", reply.resp)
			h := newHarness(t, transport)
			h.waitForText("octo-org/sample-repo#7")

			h.press("r")
			screen := h.waitForText("stale")
			for _, want := range []string{"Couldn't refresh", reply.want, "octo-org/sample-repo#7", "Untagged 3"} {
				if !strings.Contains(screen, want) {
					t.Errorf("screen is missing %q:\n%s", want, screen)
				}
			}
		})
	}
}

func TestSuccessfulRefreshClearsStale(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	transport.Reply("SearchPullRequests", githubtest.Response{Status: 502, Body: []byte(`{"message":"Server Error"}`)})
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")

	h.press("r")
	h.waitForText("stale")
	h.press("r")
	h.waitForScreen("stale marker to clear", func(s string) bool {
		return !strings.Contains(s, "stale") && strings.Contains(s, "octo-org/sample-repo#7")
	})
}

func TestStatusBarWarnsWhenRateLimitIsLow(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_status.json"))
	transport.ReplyFixture(t, "CheckContexts", fixture("check_contexts_page_2.json"))
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_low_rate_limit.json"))
	h := newHarness(t, transport)

	screen := h.waitForText("octo-org/sample-repo#12")
	if strings.Contains(screen, "rate limit") {
		t.Errorf("a healthy budget should not be shown:\n%s", screen)
	}

	h.press("r")
	h.waitForText("rate limit 312/5000 left")
}
