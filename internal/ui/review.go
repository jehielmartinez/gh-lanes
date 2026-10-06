package ui

import (
	"fmt"
	"math"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// tab is one of the two views the tab bar switches between.
type tab int

const (
	tabBoard tab = iota
	tabReview
	tabCount
)

// tabBarLines is the height of the tab bar at the top of the screen: each
// tab's box, and the line the boxes stand on.
const tabBarLines = 3

// reviewListWidth is the widest the review requests list grows; one column of
// cards wider than this is harder to read, not easier.
const reviewListWidth = 2 * maxLaneWidth

// tabLabels are the tab bar's labels, in tab order.
func (m Model) tabLabels() []string {
	review := "Review requests"
	if m.loaded {
		review += fmt.Sprintf(" %d", len(m.reviews))
	}
	return []string{"Board", review}
}

// tabBorder is a tab's box. The tab in view is open at the bottom, so it
// joins the content beneath; the others close onto the shared line.
func tabBorder(active, first bool) lipgloss.Border {
	b := lipgloss.RoundedBorder()
	b.BottomLeft, b.Bottom, b.BottomRight = "┴", "─", "┴"
	if active {
		b.BottomLeft, b.Bottom, b.BottomRight = "┘", " ", "└"
	}
	if first {
		b.BottomLeft = "├"
		if active {
			b.BottomLeft = "│"
		}
	}
	return b
}

func (m Model) tabBoxes() []string {
	labels := m.tabLabels()
	boxes := make([]string, len(labels))
	for i, label := range labels {
		active := tab(i) == m.tab
		border, text := m.theme.border, lipgloss.NewStyle().Foreground(m.theme.muted)
		if active {
			border, text = m.theme.accent, lipgloss.NewStyle().Bold(true).Foreground(m.theme.accent)
		}
		boxes[i] = lipgloss.NewStyle().
			Border(tabBorder(active, i == 0)).
			BorderForeground(border).
			Padding(0, 1).
			Render(text.Render(label))
	}
	return boxes
}

func (m Model) tabsView() string {
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, m.tabBoxes()...)
	rest := m.width - lipgloss.Width(row)
	if rest <= 0 {
		return row
	}
	// The line the tabs stand on runs on to the edge of the screen.
	lines := strings.Split(row, "\n")
	pad := strings.Repeat(" ", rest)
	for i := range lines[:len(lines)-1] {
		lines[i] += pad
	}
	lines[len(lines)-1] += lipgloss.NewStyle().Foreground(m.theme.border).Render(strings.Repeat("─", rest))
	return strings.Join(lines, "\n")
}

// tabAt is the tab whose box is drawn at screen cell x, y.
func (m Model) tabAt(x, y int) (tab, bool) {
	if y < 0 || y >= tabBarLines {
		return 0, false
	}
	start := 0
	for i, box := range m.tabBoxes() {
		end := start + lipgloss.Width(box)
		if x >= start && x < end {
			return tab(i), true
		}
		start = end
	}
	return 0, false
}

// switchTab moves to the tab delta places along, wrapping around.
func (m Model) switchTab(delta int) Model {
	m.tab = (m.tab + tab(delta) + tabCount) % tabCount
	m.lastClick = click{}
	return m
}

// selectedReview is the review request under the cursor.
func (m Model) selectedReview() (domain.PullRequest, bool) {
	if len(m.reviews) == 0 {
		return domain.PullRequest{}, false
	}
	return m.reviews[m.reviewCursor], true
}

// current is the pull request selected on the tab in view.
func (m Model) current() (domain.PullRequest, bool) {
	if m.tab == tabReview {
		return m.selectedReview()
	}
	return m.selected()
}

func (m Model) withReviewCursor(index int) Model {
	m.reviewCursor = max(0, min(index, len(m.reviews)-1))
	return m
}

// withReviews replaces the review requests, most recently updated first,
// keeping the selection on the pull request it was on.
func (m Model) withReviews(prs []domain.PullRequest) Model {
	selected, hadSelection := m.selectedReview()
	m.reviews = board.ByUpdated(prs)
	m = m.withReviewCursor(m.reviewCursor)
	if !hadSelection {
		return m
	}
	for i, pr := range m.reviews {
		if pr.ID == selected.ID {
			return m.withReviewCursor(i)
		}
	}
	return m
}

func (m Model) reviewKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.CardUp):
		return m.withReviewCursor(m.reviewCursor - 1), nil
	case key.Matches(msg, m.keys.CardDown):
		return m.withReviewCursor(m.reviewCursor + 1), nil
	case key.Matches(msg, m.keys.Open):
		if pr, ok := m.selectedReview(); ok {
			return m.openDetail(pr)
		}
	}
	return m, nil
}

// reviewWidth is the width of the review requests list.
func (m Model) reviewWidth() int {
	if m.width == 0 {
		return reviewListWidth
	}
	return max(minLaneWidth, min(reviewListWidth, m.width))
}

// reviewCardsInView is how many review requests show at once; always at
// least one.
func (m Model) reviewCardsInView() int {
	if m.height == 0 {
		return math.MaxInt32
	}
	return max(1, m.boardHeight()/cardHeight)
}

// reviewView draws the review requests in view from the scroll offset.
func (m Model) reviewView() string {
	width := m.reviewWidth()
	if m.loaded && len(m.reviews) == 0 {
		return lipgloss.NewStyle().Foreground(m.theme.muted).Render("No review requests.")
	}
	var rows []string
	last := min(len(m.reviews), m.reviewOffset+m.reviewCardsInView())
	for i := m.reviewOffset; i < last; i++ {
		rows = append(rows, m.cardView(m.reviews[i], width, i == m.reviewCursor, false))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// reviewCardAt is the index of the review request drawn at screen cell x, y.
func (m Model) reviewCardAt(x, y int) (int, bool) {
	y -= tabBarLines
	if x < 0 || x >= m.reviewWidth() || y < 0 {
		return 0, false
	}
	if height := m.boardHeight(); height > 0 && y >= height {
		return 0, false
	}
	row := y / cardHeight
	card := m.reviewOffset + row
	if row >= m.reviewCardsInView() || card >= len(m.reviews) {
		return 0, false
	}
	return card, true
}

// clickReview selects the review request under a left click, and opens it on
// a double click.
func (m Model) clickReview(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	card, ok := m.reviewCardAt(msg.X, msg.Y)
	if msg.Button != tea.MouseLeft || !ok {
		m.lastClick = click{}
		return m, nil
	}
	m = m.withReviewCursor(card)
	return m.clickedCard(m.reviews[card])
}

// reviewWheeled scrolls the review requests by one card, taking the selection
// along so it stays in view.
func (m Model) reviewWheeled(msg tea.MouseWheelMsg) Model {
	delta := wheelDelta(msg)
	if _, ok := m.reviewCardAt(msg.X, msg.Y); !ok || delta == 0 {
		return m
	}
	size := m.reviewCardsInView()
	m.reviewOffset = max(0, min(m.reviewOffset+delta, len(m.reviews)-size))
	return m.withReviewCursor(max(m.reviewOffset, min(m.reviewCursor, m.reviewOffset+size-1)))
}

// reviewHelp is the help footer on the review requests tab, which has no
// lanes to move between.
type reviewHelp struct{ keys keyMap }

func (h reviewHelp) ShortHelp() []key.Binding {
	k := h.keys
	return []key.Binding{k.CardUp, k.CardDown, k.Open, k.NextTab, k.UpdateBranch, k.Draft, k.Help, k.Quit}
}

func (h reviewHelp) FullHelp() [][]key.Binding {
	k := h.keys
	return [][]key.Binding{
		{k.CardUp, k.CardDown, k.Open, k.NextTab, k.PrevTab},
		{k.UpdateBranch, k.RebaseBranch, k.Draft},
		{k.Tags, k.Refresh, k.Help, k.Quit},
	}
}
