package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// tab is one of the views the tab bar switches between.
type tab int

const (
	tabBoard tab = iota
	tabReview
	tabArchived
	tabCount
)

// tabBarLines is the height of the tab bar at the top of the screen: each
// tab's box, and the line the boxes stand on.
const tabBarLines = 3

// tabLabels are the tab bar's labels, in tab order.
func (m Model) tabLabels() []string {
	review, archived := "Review requests", "Archived"
	if m.loaded {
		review += fmt.Sprintf(" %d", len(m.reviews.prs))
	}
	if m.storeReady {
		archived += fmt.Sprintf(" %d", len(m.archived))
	}
	return []string{"Board", review, archived}
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

// switchTab moves to the tab delta places along, wrapping around, and loads
// the archived pull requests when that tab comes into view.
func (m Model) switchTab(delta int) (Model, tea.Cmd) {
	m.tab = (m.tab + tab(delta) + tabCount) % tabCount
	m.lastClick = click{}
	if m.tab == tabArchived {
		return m.fetchArchived()
	}
	return m, nil
}

// current is the pull request selected on the tab in view.
func (m Model) current() (domain.PullRequest, bool) {
	if m.tab == tabBoard {
		return m.selected()
	}
	return m.list(m.tab).selected()
}

// listHelp is the help footer on the review requests and archived tabs,
// which have no lanes to move between.
type listHelp struct {
	keys     keyMap
	archived bool
}

func (h listHelp) ShortHelp() []key.Binding {
	k := h.keys
	if h.archived {
		return []key.Binding{k.CardUp, k.CardDown, k.Open, k.Unarchive, k.NextTab, k.Help, k.Quit}
	}
	return []key.Binding{k.CardUp, k.CardDown, k.Open, k.NextTab, k.UpdateBranch, k.Draft, k.Help, k.Quit}
}

func (h listHelp) FullHelp() [][]key.Binding {
	k := h.keys
	move := []key.Binding{k.CardUp, k.CardDown, k.Open, k.NextTab, k.PrevTab}
	if h.archived {
		move = append(move, k.Unarchive)
	}
	return [][]key.Binding{
		move,
		{k.UpdateBranch, k.RebaseBranch, k.Draft},
		{k.Tags, k.Refresh, k.Help, k.Quit},
	}
}
