package ui_test

import (
	"strings"
	"testing"
	"time"
)

var sectionTitles = []string{"Status", "Checks", "Description", "Conversation"}

// expandSections opens every collapsed section of the modal by its number
// key, once the fresh copy has drawn them all, and waits until none is
// closed.
func (h *harness) expandSections() string {
	h.t.Helper()
	screen := h.waitForText(" Conversation")
	for i, title := range sectionTitles {
		if strings.Contains(screen, "▸ "+title) {
			h.press(string(rune('1' + i)))
		}
	}
	return h.waitForScreen("every section to open", func(s string) bool {
		for _, title := range sectionTitles {
			if !strings.Contains(s, "▾ "+title) {
				return false
			}
		}
		return true
	})
}

func TestModalOpensWithStatusAndFailingChecksExpanded(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")
	h.resize(120, 60)

	h.doubleClickCard("octo-org/sample-repo#11")
	screen := h.waitForText("▸ Conversation")
	assertOrder(t, screen, "▾ Status", "▾ Checks", "✗ integration", "▸ Description", "▸ Conversation")
}

func TestDescriptionAndConversationStartCollapsed(t *testing.T) {
	h := openConversationHarness(t)

	screen := h.waitForText("▸ Conversation")
	assertOrder(t, screen, "▾ Status", "▸ Checks  No checks", "▸ Description", "▸ Conversation")
	for _, hidden := range []string{"pins it by digest.", "Mention the new tag."} {
		if strings.Contains(screen, hidden) {
			t.Errorf("collapsed section shows %q:\n%s", hidden, screen)
		}
	}
}

func TestPassingChecksStartCollapsed(t *testing.T) {
	h := openLinksHarness(t, 120, 120)

	screen := h.waitForText("▸ Checks")
	if strings.Contains(screen, "✓ build") {
		t.Errorf("checks that all passed should be collapsed:\n%s", screen)
	}
}

func TestCollapsedSectionsSummariseWhatTheyHide(t *testing.T) {
	h := openConversationHarness(t)

	screen := h.waitForText("▸ Conversation")
	assertContains(t, screen,
		"▸ Description  ",
		"Summary",
		"▸ Conversation  ",
		"▸ Conversation  7 · latest ghost 2h ago",
	)
	h.press("1")
	screen = h.waitForText("▸ Status")
	assertContains(t, screen, "▸ Status  ")
}

func TestNumberKeysToggleSections(t *testing.T) {
	h := openConversationHarness(t)
	h.waitForText("▸ Conversation")

	h.press("3")
	h.waitForText("pins it by digest.")
	h.press("4")
	h.waitForText("Mention the new tag.")
	h.press("3")
	h.waitForScreen("the description to close", func(s string) bool {
		return strings.Contains(s, "▸ Description") && !strings.Contains(s, "pins it by digest.")
	})
	h.press("2")
	h.waitForText("▸ Checks")
}

func TestClickingASectionHeadingTogglesIt(t *testing.T) {
	h := openConversationHarness(t)

	h.clickText("▸ Description")
	h.waitForText("pins it by digest.")
	h.clickText("▾ Description")
	h.waitForScreen("the description to close", func(s string) bool {
		return !strings.Contains(s, "pins it by digest.")
	})
}

func TestRefreshKeepsSectionsAsTheViewerLeftThem(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json", "detail_conflict_refreshed.json")
	h.resize(120, 60)

	h.doubleClickCard("octo-org/sample-repo#11")
	h.waitForText("▸ Conversation")
	h.press("2")
	h.press("3")
	h.waitForScreen("checks closed and the description open", func(s string) bool {
		return strings.Contains(s, "▸ Checks") && strings.Contains(s, "▾ Description")
	})

	h.settle(time.Minute)
	h.waitForDetailRequests(2)
	screen := h.waitForText("6 passed · 1 failed · 2 running · 1 skipped")
	assertContains(t, screen, "▸ Checks", "▾ Description", "▸ Conversation")
}

func TestHelpListsTheSectionKeys(t *testing.T) {
	h := newConversationHarness(t)

	assertContains(t, h.waitForText("1-4 sections"), "1-4 sections")
}

func TestSectionHeadingsHintTheirKey(t *testing.T) {
	h := openConversationHarness(t)

	screen := h.waitForText("▸ Conversation")
	assertContains(t, screen, "1 close", "2 open", "3 open", "4 open")
	h.press("3")
	h.waitForText("3 close")
}

func TestNarrowHeadingsDropTheSummaryBeforeTheHint(t *testing.T) {
	h := openConversationHarness(t)
	h.resize(40, 60)

	screen := h.waitForText("4 open")
	if !strings.Contains(screen, "▸ Conversation") {
		t.Errorf("the title should survive a narrow modal:\n%s", screen)
	}
}

func TestResolvedThreadsHintTheExpandKey(t *testing.T) {
	h := newConversationHarness(t)

	h.waitForText("Resolved · 2 comments · e to expand")
	h.press("e")
	h.waitForText("e to collapse")
}
