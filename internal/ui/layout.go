package ui

import (
	"math"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Lanes share the terminal's width between these bounds; below the minimum
// the board scrolls sideways instead of squeezing them.
const (
	minLaneWidth = 32
	maxLaneWidth = 36
	laneGap      = 1
)

const (
	// laneHeaderLines is a lane's name and count, and a blank line that
	// sets them apart from the lane's first group or card.
	laneHeaderLines = 2
	cardLines       = 4
	// cardHeight is a card's lines plus its top and bottom border.
	cardHeight = cardLines + 2
)

// laneWidth is the width of every lane, gap excluded.
func (m Model) laneWidth() int {
	if m.width == 0 {
		return maxLaneWidth
	}
	return max(minLaneWidth, min(maxLaneWidth, (m.width+laneGap)/len(m.lanes)-laneGap))
}

// lanesInView is how many lanes fit side by side; always at least one.
func (m Model) lanesInView() int {
	if m.width == 0 {
		return len(m.lanes)
	}
	return max(1, (m.width+laneGap)/(m.laneWidth()+laneGap))
}

// boardHeight is the rows left for the board, or the review requests list,
// between the tab bar and the footer, or 0 while the terminal's size is
// unknown.
func (m Model) boardHeight() int {
	if m.height == 0 {
		return 0
	}
	return max(0, m.height-tabBarLines-lipgloss.Height(m.footerView()))
}

// laneSpace is the lines a lane has for its rows, under its header.
func (m Model) laneSpace() int {
	if m.height == 0 {
		return math.MaxInt32
	}
	return m.boardHeight() - laneHeaderLines
}

// scrolled moves the board and every lane just far enough that the focused
// lane and each lane's selected card are in view.
func (m Model) scrolled() Model {
	m.firstLane = window(m.firstLane, m.focus, m.lanesInView(), len(m.lanes))
	space := m.laneSpace()
	offsets := make([]int, len(m.lanes))
	for i, lane := range m.lanes {
		offsets[i] = scrollTo(laneRows(lane), m.offsets[i], m.cursors[i], space)
	}
	m.offsets = offsets
	m.reviews = m.reviews.scrolled(m.listSpace())
	m.archive = m.archive.scrolled(m.listSpace())
	return m
}

// window returns the first visible index of a run of size items out of
// total, starting from offset and moved as little as possible to show index.
func window(offset, index, size, total int) int {
	offset = min(offset, index)
	offset = max(offset, index-size+1)
	offset = min(offset, total-size)
	return max(offset, 0)
}

// laneAt is the lane drawn at screen cell x, y.
func (m Model) laneAt(x, y int) (int, bool) {
	y -= tabBarLines
	if board := m.boardHeight(); y < 0 || (board > 0 && y >= board) {
		return 0, false
	}
	step := m.laneWidth() + laneGap
	column := x / step
	lane := m.firstLane + column
	if x < 0 || x%step >= m.laneWidth() || column >= m.lanesInView() || lane >= len(m.lanes) {
		return 0, false
	}
	return lane, true
}

// cardAt is the card drawn at screen cell x, y.
func (m Model) cardAt(x, y int) (lane, card int, ok bool) {
	lane, ok = m.laneAt(x, y)
	y -= tabBarLines + laneHeaderLines
	if !ok || y < 0 {
		return 0, 0, false
	}
	card, ok = cardAtLine(laneRows(m.lanes[lane]), m.offsets[lane], m.laneSpace(), y)
	return lane, card, ok
}

// clicked selects the card under a left click.
func (m Model) clicked(msg tea.MouseClickMsg) Model {
	lane, card, ok := m.cardAt(msg.X, msg.Y)
	if msg.Button != tea.MouseLeft || !ok {
		return m
	}
	m.focus = lane
	return m.withCursor(lane, card)
}

// wheeled scrolls the lane under the pointer by one row, taking its
// selection along so the selected card stays in view.
func (m Model) wheeled(msg tea.MouseWheelMsg) Model {
	delta := wheelDelta(msg)
	lane, ok := m.laneAt(msg.X, msg.Y)
	if !ok || delta == 0 {
		return m
	}
	offset, cursor := wheel(laneRows(m.lanes[lane]), m.offsets[lane], m.cursors[lane], m.laneSpace(), delta)
	offsets := slices.Clone(m.offsets)
	offsets[lane] = offset
	m.offsets = offsets
	return m.withCursor(lane, cursor)
}

// wheelDelta is how many rows a turn of the wheel scrolls by.
func wheelDelta(msg tea.MouseWheelMsg) int {
	switch msg.Button {
	case tea.MouseWheelUp:
		return -1
	case tea.MouseWheelDown:
		return 1
	}
	return 0
}
