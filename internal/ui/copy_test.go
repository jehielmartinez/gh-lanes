package ui_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"slices"
	"testing"

	"github.com/charmbracelet/x/exp/teatest/v2"
)

func TestYCopiesThePullRequestsURL(t *testing.T) {
	h := newHarness(t, linksTransport(t))
	h.waitForText("octo-org/sample-repo#12")

	h.press("y")
	h.waitForCopied(selectedPR)
	h.waitForText("Copied " + selectedPR)

	h.press("enter")
	h.waitForText("Document the release steps")
	h.press("y")
	h.waitForCopied(selectedPR, linksPR)
	h.waitForText("Copied " + linksPR)
}

func TestYCopiesTheSelectedReviewRequest(t *testing.T) {
	h := newReviewHarness(t, nil)
	h.press("tab")
	h.waitForText(openReviewRef)

	h.press("j")
	h.press("y")
	h.waitForCopied("https://github.com/octo-org/sample-repo/pull/31")
}

func TestYCopiesTheSelectedArchivedPR(t *testing.T) {
	h := startArchived(t, `[{"id":"PR_node_merged","open":false,"tag":"testing"}]`)

	h.clickText("Archived 1")
	h.waitForText("Ship the upload worker")
	h.press("y")
	h.waitForCopied("https://github.com/octo-org/sample-repo/pull/20")
}

func TestYFallsBackToTheTerminalClipboard(t *testing.T) {
	h := newHarness(t, linksTransport(t), withCopyError(errors.New("no clipboard tool found")))
	h.waitForText("octo-org/sample-repo#12")

	h.press("y")
	h.waitForText("Sent " + selectedPR + " to terminal clipboard")
	osc52 := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(selectedPR)) + "\x07"
	teatest.WaitFor(t, h.tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte(osc52))
	}, teatest.WithDuration(waitTimeout))
}

func TestYDoesNothingInTheLinkPicker(t *testing.T) {
	h := newLinksHarness(t, 160, 60)

	h.press("o")
	h.waitForText("description · user-a")
	h.press("y")
	h.press("esc")
	h.waitForScreen("the picker to close", func(s string) bool { return !onScreen(s, "description · user-a") })
	h.press("y")
	h.waitForCopied(linksPR)
	// Opening runs after both copies were asked for, so any copy the picker
	// let through has landed by the time the opener gets the URL.
	h.press("O")
	h.waitForOpened(linksPR)
	if got := h.clipboard.copied(); !slices.Equal(got, []string{linksPR}) {
		t.Fatalf("y in the link picker should copy nothing; the clipboard got %q", got)
	}
}

func TestFullHelpListsCopyLink(t *testing.T) {
	h := newHarness(t, linksTransport(t))
	h.waitForText("octo-org/sample-repo#12")

	h.press("?")
	h.waitForText("copy link")
}
