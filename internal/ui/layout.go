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
	laneHeaderLines = 1
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

// cardsInView is how many cards a lane shows at once; always at least one.
func (m Model) cardsInView() int {
	if m.height == 0 {
		return math.MaxInt32
	}
	return max(1, (m.boardHeight()-laneHeaderLines)/cardHeight)
}

// scrolled moves the board and every lane just far enough that the focused
// lane and each lane's selected card are in view.
func (m Model) scrolled() Model {
	m.firstLane = window(m.firstLane, m.focus, m.lanesInView(), len(m.lanes))
	size := m.cardsInView()
	offsets := make([]int, len(m.lanes))
	for i, lane := range m.lanes {
		offsets[i] = window(m.offsets[i], m.cursors[i], size, len(lane.PullRequests))
	}
	m.offsets = offsets
	m.reviews = m.reviews.scrolled(m.listCardsInView())
	m.archive = m.archive.scrolled(m.listCardsInView())
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
	y -= tabBarLines
	row := (y - laneHeaderLines) / cardHeight
	if !ok || y < laneHeaderLines || row >= m.cardsInView() {
		return 0, 0, false
	}
	card = m.offsets[lane] + row
	if card >= len(m.lanes[lane].PullRequests) {
		return 0, 0, false
	}
	return lane, card, true
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

// wheeled scrolls the lane under the pointer by one card, taking its
// selection along so the selected card stays in view.
func (m Model) wheeled(msg tea.MouseWheelMsg) Model {
	delta := wheelDelta(msg)
	lane, ok := m.laneAt(msg.X, msg.Y)
	if !ok || delta == 0 {
		return m
	}
	size, total := m.cardsInView(), len(m.lanes[lane].PullRequests)
	offset := max(0, min(m.offsets[lane]+delta, total-size))
	offsets := slices.Clone(m.offsets)
	offsets[lane] = offset
	m.offsets = offsets
	return m.withCursor(lane, max(offset, min(m.cursors[lane], offset+size-1)))
}

// wheelDelta is how many cards a turn of the wheel scrolls by.
func wheelDelta(msg tea.MouseWheelMsg) int {
	switch msg.Button {
	case tea.MouseWheelUp:
		return -1
	case tea.MouseWheelDown:
		return 1
	}
	return 0
}
