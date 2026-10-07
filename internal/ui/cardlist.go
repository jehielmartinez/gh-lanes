package ui

import (
	"math"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// listWidth is the widest a single column of cards grows; one column wider
// than this is harder to read, not easier.
const listWidth = 2 * maxLaneWidth

// cardList is one column of cards, most recently updated first, with a
// selection and a scroll offset: the review requests and archived tabs.
type cardList struct {
	prs    []domain.PullRequest
	cursor int
	offset int
}

func (l cardList) selected() (domain.PullRequest, bool) {
	if len(l.prs) == 0 {
		return domain.PullRequest{}, false
	}
	return l.prs[l.cursor], true
}

func (l cardList) withCursor(index int) cardList {
	l.cursor = max(0, min(index, len(l.prs)-1))
	return l
}

// withPRs replaces the cards, most recently updated first, keeping the
// selection on the pull request it was on.
func (l cardList) withPRs(prs []domain.PullRequest) cardList {
	selected, hadSelection := l.selected()
	l.prs = board.ByUpdated(prs)
	l = l.withCursor(l.cursor)
	if !hadSelection {
		return l
	}
	for i, pr := range l.prs {
		if pr.ID == selected.ID {
			return l.withCursor(i)
		}
	}
	return l
}

// withCopy replaces the list's copy of a pull request, if it has one.
func (l cardList) withCopy(pr domain.PullRequest) cardList {
	for _, have := range l.prs {
		if have.ID == pr.ID {
			return l.withPRs(replaced(l.prs, pr))
		}
	}
	return l
}

// scrolled moves the offset just far enough to show the selection, size
// cards at a time.
func (l cardList) scrolled(size int) cardList {
	l.offset = window(l.offset, l.cursor, size, len(l.prs))
	return l
}

// list is the card list on a tab other than the board.
func (m Model) list(t tab) cardList {
	if t == tabArchived {
		return m.archive
	}
	return m.reviews
}

func (m Model) withList(t tab, l cardList) Model {
	if t == tabArchived {
		m.archive = l
	} else {
		m.reviews = l
	}
	return m
}

// withListCopies hands a fresher copy of a pull request to every card list
// that shows it.
func (m Model) withListCopies(pr domain.PullRequest) Model {
	m.requested = replaced(m.requested, pr)
	m.reviews = m.reviews.withCopy(pr)
	m.archivePRs = replaced(m.archivePRs, pr)
	m.archive = m.archive.withCopy(pr)
	return m
}

func (m Model) listKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	l := m.list(m.tab)
	switch {
	case key.Matches(msg, m.keys.CardUp):
		return m.withList(m.tab, l.withCursor(l.cursor-1)), nil
	case key.Matches(msg, m.keys.CardDown):
		return m.withList(m.tab, l.withCursor(l.cursor+1)), nil
	case key.Matches(msg, m.keys.Open):
		if pr, ok := l.selected(); ok {
			return m.openDetail(pr)
		}
	case m.tab == tabArchived && key.Matches(msg, m.keys.Unarchive):
		return m.unarchiveSelected()
	case m.tab == tabReview && key.Matches(msg, m.keys.Archive):
		return m.archiveReviewRequest()
	}
	return m, nil
}

// listColumnWidth is the width of a card list.
func (m Model) listColumnWidth() int {
	if m.width == 0 {
		return listWidth
	}
	return max(minLaneWidth, min(listWidth, m.width))
}

// listCardsInView is how many cards of a list show at once; always at least
// one.
func (m Model) listCardsInView() int {
	if m.height == 0 {
		return math.MaxInt32
	}
	return max(1, m.boardHeight()/cardHeight)
}

// listView draws the cards of l in view from its scroll offset, or note
// when there are none.
func (m Model) listView(l cardList, note string) string {
	if note != "" {
		return lipgloss.NewStyle().Foreground(m.theme.muted).Render(note)
	}
	width := m.listColumnWidth()
	var rows []string
	last := min(len(l.prs), l.offset+m.listCardsInView())
	for i := l.offset; i < last; i++ {
		rows = append(rows, m.cardView(l.prs[i], width, i == l.cursor, false))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// listCardAt is the index of the card in l drawn at screen cell x, y.
func (m Model) listCardAt(l cardList, x, y int) (int, bool) {
	y -= tabBarLines
	if x < 0 || x >= m.listColumnWidth() || y < 0 {
		return 0, false
	}
	if height := m.boardHeight(); height > 0 && y >= height {
		return 0, false
	}
	row := y / cardHeight
	card := l.offset + row
	if row >= m.listCardsInView() || card >= len(l.prs) {
		return 0, false
	}
	return card, true
}

// clickList selects the card under a left click, and opens it on a double
// click.
func (m Model) clickList(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	l := m.list(m.tab)
	card, ok := m.listCardAt(l, msg.X, msg.Y)
	if msg.Button != tea.MouseLeft || !ok {
		m.lastClick = click{}
		return m, nil
	}
	m = m.withList(m.tab, l.withCursor(card))
	return m.clickedCard(l.prs[card])
}

// listWheeled scrolls the list by one card, taking the selection along so it
// stays in view.
func (m Model) listWheeled(msg tea.MouseWheelMsg) Model {
	l := m.list(m.tab)
	delta := wheelDelta(msg)
	if _, ok := m.listCardAt(l, msg.X, msg.Y); !ok || delta == 0 {
		return m
	}
	size := m.listCardsInView()
	l.offset = max(0, min(l.offset+delta, len(l.prs)-size))
	return m.withList(m.tab, l.withCursor(max(l.offset, min(l.cursor, l.offset+size-1))))
}
