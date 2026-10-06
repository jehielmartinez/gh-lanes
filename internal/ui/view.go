package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

const (
	laneWidth = 36
	// cardPadding is the border plus one column of padding on each side.
	cardPadding = 4
	footerLines = 2
)

// View draws the board, the status bar and the help footer.
func (m Model) View() tea.View {
	body := m.boardView()
	if m.height > footerLines {
		body = lipgloss.NewStyle().Height(m.height - footerLines).MaxHeight(m.height - footerLines).Render(body)
	}
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, m.statusView(), m.help.View(m.keys)))
	v.AltScreen = true
	return v
}

func (m Model) boardView() string {
	cols := make([]string, 0, len(m.lanes))
	for _, lane := range m.lanes {
		cols = append(cols, m.laneView(lane))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) laneView(lane board.Lane) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(m.theme.untagged).Render(lane.Name) +
		" " + lipgloss.NewStyle().Foreground(m.theme.muted).Render(fmt.Sprint(len(lane.PullRequests)))
	rows := []string{header}
	if !m.loading && m.err == nil && len(lane.PullRequests) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.muted).Render("No open pull requests."))
	}
	for _, pr := range lane.PullRequests {
		rows = append(rows, m.cardView(pr))
	}
	return lipgloss.NewStyle().Width(laneWidth).MarginRight(1).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) cardView(pr domain.PullRequest) string {
	inner := laneWidth - cardPadding
	number := fmt.Sprintf("#%d", pr.Number)
	ref := truncate(pr.Repository, inner-lipgloss.Width(number)) + number
	ref = lipgloss.NewStyle().Foreground(m.theme.muted).Render(ref)
	title := lipgloss.NewStyle().Foreground(m.theme.text).Render(truncate(oneLine(pr.Title), inner))
	return lipgloss.NewStyle().
		Width(laneWidth).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.border).
		Render(ref + "\n" + title)
}

func (m Model) statusView() string {
	switch {
	case m.err != nil:
		return lipgloss.NewStyle().Foreground(m.theme.errText).Render("Couldn't load pull requests: " + oneLine(m.err.Error()))
	case m.loading:
		return lipgloss.NewStyle().Foreground(m.theme.muted).Render("Loading pull requests…")
	}
	return ""
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate shortens s to at most width cells, ending in an ellipsis when it
// had to cut.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)) > width-1 {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}
