package ui

import "github.com/jehielmartinez/gh-lanes/internal/board"

// row is one entry of a lane or card list as it is laid out top to bottom.
// Scrolling, keeping the selection in view and hit-testing all count in
// rows, so rows of different heights can share a lane.
type row struct {
	height int
	// card is the index of the card the row draws, in its lane or list, or
	// noCard for a display-only row that clicks and the selection pass over.
	card int
	// group is the index of the group whose header a noCard row draws.
	group int
}

const (
	noCard = -1
	// groupHeaderHeight is the one line of a group's name and size.
	groupHeaderHeight = 1
)

// cardRows lays out n cards, one row each.
func cardRows(n int) []row {
	rows := make([]row, n)
	for i := range rows {
		rows[i] = row{height: cardHeight, card: i}
	}
	return rows
}

func laneRows(lane board.Lane) []row {
	return groupedRows(lane.Groups, len(lane.PullRequests))
}

func (l cardList) rows() []row {
	return groupedRows(l.groups, len(l.prs))
}

// groupedRows lays out n cards, each group's under its header, or one row
// per card when there are no groups.
func groupedRows(groups []board.Group, n int) []row {
	if len(groups) == 0 {
		return cardRows(n)
	}
	rows := make([]row, 0, len(groups)+n)
	card := 0
	for g, group := range groups {
		rows = append(rows, row{height: groupHeaderHeight, card: noCard, group: g})
		for range group.Size {
			rows = append(rows, row{height: cardHeight, card: card})
			card++
		}
	}
	return rows
}

// rowsInView is how many rows from first fit in space lines; always at
// least one while any remain, so a terminal too short for a whole row still
// shows the top of one.
func rowsInView(rows []row, first, space int) int {
	n, used := 0, 0
	for _, r := range rows[min(first, len(rows)):] {
		if n > 0 && used+r.height > space {
			break
		}
		used += r.height
		n++
	}
	return n
}

// inView is the rows drawn from first in space lines.
func inView(rows []row, first, space int) []row {
	first = min(first, len(rows))
	return rows[first : first+rowsInView(rows, first, space)]
}

// maxFirst is the furthest a run of rows scrolls: the first row in view
// once the last row is, with as many rows above it as space allows.
func maxFirst(rows []row, space int) int {
	first, used := len(rows), 0
	for first > 0 && used+rows[first-1].height <= space {
		first--
		used += rows[first].height
	}
	if first == len(rows) {
		return max(len(rows)-1, 0)
	}
	return first
}

// rowOf is the index of the row that draws card, or of the last row when no
// row does.
func rowOf(rows []row, card int) int {
	for i, r := range rows {
		if r.card == card {
			return i
		}
	}
	return max(len(rows)-1, 0)
}

// scrollTo returns the first row in view, starting from first and moved as
// little as possible to show card in full in space lines, and never so far
// down that space is left empty under the last row. Scrolling up to a card
// also brings the header row just above it into view.
func scrollTo(rows []row, first, card, space int) int {
	if len(rows) == 0 {
		return 0
	}
	target := rowOf(rows, card)
	top := target
	if top > 0 && rows[top-1].card == noCard {
		top--
	}
	first = min(first, top)
	for first < target && target >= first+rowsInView(rows, first, space) {
		first++
	}
	return min(first, maxFirst(rows, space))
}

// cardAtLine is the card drawn at line y of rows laid out in space lines
// from row first.
func cardAtLine(rows []row, first, space, y int) (int, bool) {
	if y < 0 {
		return 0, false
	}
	for _, r := range inView(rows, first, space) {
		if y < r.height {
			return r.card, r.card != noCard
		}
		y -= r.height
	}
	return 0, false
}

// wheel scrolls rows laid out in space lines by delta rows from first, and
// moves cursor, a card index, just enough to keep it on a card in view.
func wheel(rows []row, first, cursor, space, delta int) (newFirst, newCursor int) {
	first = max(0, min(first+delta, maxFirst(rows, space)))
	top, bottom := noCard, noCard
	for _, r := range inView(rows, first, space) {
		if r.card == noCard {
			continue
		}
		if top == noCard {
			top = r.card
		}
		bottom = r.card
	}
	if top == noCard {
		return first, cursor
	}
	return first, max(top, min(cursor, bottom))
}
