package ui_test

import (
	"strings"
	"testing"
)

func TestTabsAreBoxesWithTheActiveOneOpenBelow(t *testing.T) {
	h := startBoard(t)

	rows := strings.Split(h.screen.plain(), "\n")[:tabBarRows]
	assertContains(t, rows[0], "╭───────╮")
	assertContains(t, rows[1], "│ Board │", "Review requests")
	// The active tab has no bottom edge, so it opens onto the board.
	if !strings.HasPrefix(rows[2], "│       └") {
		t.Errorf("the Board tab should be open at the bottom:\n%s", strings.Join(rows, "\n"))
	}
}
