package ui

import "github.com/jehielmartinez/gh-lanes/internal/board"

// row is one entry of a lane or card list as it is laid out top to bottom.
// Scrolling, keeping the selection in view and hit-testing all count in
// rows, so rows of different heights can share a lane.
type row struct {
	height int
	// card is the index of the card the row draws, in its lane or list.
	card int
}

// cardRows lays out n cards, one row each.
func cardRows(n int) []row {
	rows := make([]row, n)
	for i := range rows {
		rows[i] = row{height: cardHeight, card: i}
	}
	return rows
}

func laneRows(lane board.Lane) []row {
	return cardRows(len(lane.PullRequests))
}

func (l cardList) rows() []row {
	return cardRows(len(l.prs))
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

// lastFirst is the lowest a run of rows scrolls: the first row in view once
// the last row is, with as many rows above it as space allows.
func lastFirst(rows []row, space int) int {
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
// down that space is left empty under the last row.
func scrollTo(rows []row, first, card, space int) int {
	if len(rows) == 0 {
		return 0
	}
	target := rowOf(rows, card)
	first = min(first, target)
	for first < target && target >= first+rowsInView(rows, first, space) {
		first++
	}
	return min(first, lastFirst(rows, space))
}

// cardAtLine is the card drawn at line y of rows laid out in space lines
// from row first.
func cardAtLine(rows []row, first, space, y int) (int, bool) {
	if y < 0 {
		return 0, false
	}
	for _, r := range inView(rows, first, space) {
		if y < r.height {
			return r.card, true
		}
		y -= r.height
	}
	return 0, false
}

// wheel scrolls rows laid out in space lines by delta rows from first, and
// moves cursor, a card index, just enough to keep it on a card in view.
func wheel(rows []row, first, cursor, space, delta int) (newFirst, newCursor int) {
	first = max(0, min(first+delta, lastFirst(rows, space)))
	visible := inView(rows, first, space)
	if len(visible) == 0 {
		return first, cursor
	}
	top, bottom := visible[0].card, visible[len(visible)-1].card
	return first, max(top, min(cursor, bottom))
}
