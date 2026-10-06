package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/teatest/v2"
)

// Cards are three lines inside a border, so one terminal row of 8 leaves room
// for the lane header and a single card above the two footer lines, and 14
// for two cards.
const (
	oneCardHeight  = 8
	twoCardsHeight = 14
	narrowWidth    = 70
)

func TestNarrowTerminalScrollsLanesToKeepTheFocusedOneVisible(t *testing.T) {
	h := startBoard(t, withTermSize(narrowWidth, 30))

	header := headerLine(h.screen.plain())
	if !strings.Contains(header, "Untagged 3") || !strings.Contains(header, "In Progress 0") || strings.Contains(header, "Review") {
		t.Fatalf("a 70-column board should show the first two lanes only:\n%s", header)
	}

	h.press("l")
	h.press("l")
	header = headerLine(h.waitForScreen("Review in view", func(s string) bool {
		return strings.Contains(headerLine(s), "Review 0")
	}))
	if strings.Contains(header, "Untagged") {
		t.Errorf("the board should have scrolled past Untagged:\n%s", header)
	}

	for range 3 {
		h.press("l")
	}
	h.waitForScreen("Done in view", func(s string) bool { return strings.Contains(headerLine(s), "Done 0") })
	for range 5 {
		h.press("h")
	}
	h.waitForScreen("Untagged back in view", func(s string) bool {
		return strings.HasPrefix(headerLine(s), "Untagged 3") && !strings.Contains(headerLine(s), "Review")
	})
}

func TestLanesKeepTheirMinimumWidthInANarrowTerminal(t *testing.T) {
	h := startBoard(t, withTermSize(narrowWidth, 30))

	border := strings.Split(h.screen.plain(), "\n")[1]
	first := strings.Fields(border)[0]
	if got := len([]rune(first)); got != 32 {
		t.Errorf("card is %d columns wide, want 32:\n%s", got, h.screen.plain())
	}
}

func TestLongLaneScrollsToKeepTheSelectedCardVisible(t *testing.T) {
	h := startBoard(t, withTermSize(defaultTermWidth, twoCardsHeight))

	screen := h.screen.plain()
	if !strings.Contains(screen, "#42") || !strings.Contains(screen, "#7") || strings.Contains(screen, "#1234") {
		t.Fatalf("only the first two cards should fit:\n%s", screen)
	}

	h.press("j")
	h.press("j")
	screen = h.waitForText("#1234")
	if strings.Contains(screen, "#42") {
		t.Errorf("the lane should have scrolled the first card away:\n%s", screen)
	}

	h.press("k")
	h.press("k")
	h.waitForScreen("the first card back", func(s string) bool {
		return strings.Contains(s, "#42") && !strings.Contains(s, "#1234")
	})
}

func TestEachLaneScrollsOnItsOwn(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "state.json", `{"version":1,"assignments":{"`+newestPR+`":"in-progress"}}`)
	h := startBoard(t, withConfigDir(dir), withTermSize(defaultTermWidth, oneCardHeight))

	screen := h.waitForText("In Progress 1")
	if !strings.Contains(screen, "#7") || !strings.Contains(screen, "#42") || strings.Contains(screen, "#1234") {
		t.Fatalf("each lane should show its first card:\n%s", screen)
	}

	h.press("j")
	screen = h.waitForText("#1234")
	if strings.Contains(screen, "#7") || !strings.Contains(screen, "#42") {
		t.Errorf("only Untagged should have scrolled:\n%s", screen)
	}

	h.press("l")
	h.press("k")
	h.press("q")
	h.waitFinished()
	if screen := h.screen.plain(); !strings.Contains(screen, "#1234") {
		t.Errorf("Untagged should stay scrolled while another lane has focus:\n%s", screen)
	}
}

func TestHelpFooterIsAlwaysShownAndQuestionMarkTogglesFullHelp(t *testing.T) {
	h := startBoard(t)

	screen := h.screen.plain()
	for _, want := range []string{"? help", "q quit", "m move to…"} {
		if !strings.Contains(screen, want) {
			t.Errorf("help footer is missing %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "move right") {
		t.Errorf("the footer should be short until ? is pressed:\n%s", screen)
	}

	h.press("?")
	screen = h.waitForText("close help")
	for _, want := range []string{
		"←/h", "→/l", "↑/k", "↓/j",
		"H/<", "move left", "L/>", "move right", "m", "move to…",
		"r", "refresh", "?", "q", "quit",
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("full help is missing %q:\n%s", want, screen)
		}
	}

	h.press("?")
	h.waitForScreen("short help again", func(s string) bool {
		return strings.Contains(s, "? help") && !strings.Contains(s, "move right")
	})
}

func TestFullHelpKeepsTheSelectedCardVisible(t *testing.T) {
	h := startBoard(t, withTermSize(defaultTermWidth, twoCardsHeight))

	h.press("j")
	h.press("?")
	screen := h.waitForText("close help")
	if !strings.Contains(screen, "#7") || strings.Contains(screen, "#42") {
		t.Errorf("with less room the lane should scroll to the selected card:\n%s", screen)
	}
}

func TestMouseReportingUsesCellMotion(t *testing.T) {
	h := startBoard(t)

	const cellMotion = "\x1b[?1002h"
	teatest.WaitFor(t, h.tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte(cellMotion))
	}, teatest.WithDuration(waitTimeout))
}

func TestClickingACardSelectsIt(t *testing.T) {
	h := startBoard(t)

	h.clickText("Fix typo in README")
	h.press("L")
	h.waitForAssignments(map[string]string{olderPR: "in-progress"})

	h.clickText("#1234")
	h.press("L")
	h.waitForAssignments(map[string]string{olderPR: "in-progress", oldestPR: "in-progress"})

	h.waitForText("In Progress 2")
	h.clickText("octo-org/sample-repo#7")
	h.press("L")
	h.waitForAssignments(map[string]string{olderPR: "review", oldestPR: "in-progress"})
}

func TestClickingOutsideACardKeepsTheSelection(t *testing.T) {
	h := startBoard(t)

	h.clickText("Review 0")
	h.click(2, defaultTermHeight-4)
	h.press("L")
	h.waitForAssignments(map[string]string{newestPR: "in-progress"})
}

func TestClickingACardInAScrolledBoardSelectsThatCard(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "state.json", `{"version":1,"assignments":{"`+olderPR+`":"review","`+newestPR+`":"in-progress"}}`)
	h := newHarness(t, boardTransport(t), withConfigDir(dir), withTermSize(narrowWidth, 30))
	h.waitForText("In Progress 1")

	h.press("l")
	h.press("l")
	h.waitForScreen("In Progress and Review in view", func(s string) bool {
		return strings.HasPrefix(headerLine(s), "In Progress 1") && strings.Contains(headerLine(s), "Review 1")
	})
	h.clickText("#42")
	h.press("H")
	h.waitForAssignments(map[string]string{olderPR: "review"})
}

func TestWheelScrollsTheLaneUnderThePointer(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "state.json", `{"version":1,"assignments":{"`+newestPR+`":"in-progress"}}`)
	h := startBoard(t, withConfigDir(dir), withTermSize(defaultTermWidth, oneCardHeight))
	screen := h.waitForText("In Progress 1")
	ux, uy, _ := locate(screen, "#7")
	px, py, _ := locate(screen, "#42")

	h.wheel(ux, uy, true)
	screen = h.waitForText("#1234")
	if strings.Contains(screen, "#7") || !strings.Contains(screen, "#42") {
		t.Errorf("only the lane under the pointer should scroll:\n%s", screen)
	}

	h.wheel(px, py, true)
	h.wheel(ux, uy, false)
	h.waitForScreen("Untagged scrolled back", func(s string) bool {
		return strings.Contains(s, "#7") && strings.Contains(s, "#42") && !strings.Contains(s, "#1234")
	})

	h.wheel(ux, uy, true)
	h.waitForText("#1234")
	h.press("L")
	h.waitForAssignments(map[string]string{newestPR: "in-progress", oldestPR: "in-progress"})
}

// clickText clicks the first place text appears on screen.
func (h *harness) clickText(text string) {
	h.t.Helper()
	x, y, ok := locate(h.waitForText(text), text)
	if !ok {
		h.t.Fatalf("%q is not on screen", text)
	}
	h.click(x, y)
}
