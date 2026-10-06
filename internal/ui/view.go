package ui

import (
	"fmt"
	"image/color"
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

// View draws the board, the status bar and the help footer, with any open
// picker over them.
func (m Model) View() tea.View {
	body := m.boardView()
	if m.height > footerLines {
		body = lipgloss.NewStyle().Height(m.height - footerLines).MaxHeight(m.height - footerLines).Render(body)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, body, m.statusView(), m.help.View(m.keys))
	if m.picker != nil {
		content = m.overlay(content, m.picker.view(m.theme, m.help))
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// overlay draws fg centred on the terminal over bg.
func (m Model) overlay(bg, fg string) string {
	termW, termH := m.width, m.height
	if termW == 0 || termH == 0 {
		termW, termH = lipgloss.Width(bg), lipgloss.Height(bg)
	}
	x := max(0, (termW-lipgloss.Width(fg))/2)
	y := max(0, (termH-lipgloss.Height(fg))/2)
	canvas := lipgloss.NewCanvas(max(termW, lipgloss.Width(bg)), max(termH, lipgloss.Height(bg)))
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(bg),
		lipgloss.NewLayer(fg).X(x).Y(y).Z(1),
	))
	return canvas.Render()
}

func (m Model) boardView() string {
	cols := make([]string, 0, len(m.lanes))
	for i, lane := range m.lanes {
		cols = append(cols, m.laneView(lane, i == m.focus, m.cursors[i]))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) laneView(lane board.Lane, focused bool, cursor int) string {
	name := lipgloss.NewStyle().Bold(true).Underline(focused).Foreground(m.laneColor(lane)).Render(lane.Tag.Name)
	header := name + " " + lipgloss.NewStyle().Foreground(m.theme.muted).Render(fmt.Sprint(len(lane.PullRequests)))
	rows := []string{header}
	if lane.Untagged() && !m.loading && m.err == nil && len(m.prs) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.muted).Render("No open pull requests."))
	}
	for i, pr := range lane.PullRequests {
		rows = append(rows, m.cardView(pr, focused && i == cursor))
	}
	return lipgloss.NewStyle().Width(laneWidth).MarginRight(1).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) laneColor(lane board.Lane) color.Color {
	if lane.Tag.Color == "" {
		return m.theme.untagged
	}
	return lipgloss.Color(lane.Tag.Color)
}

func (m Model) cardView(pr domain.PullRequest, selected bool) string {
	inner := laneWidth - cardPadding
	number := fmt.Sprintf("#%d", pr.Number)
	ref := truncate(pr.Repository, inner-lipgloss.Width(number)) + number
	ref = lipgloss.NewStyle().Foreground(m.theme.muted).Render(ref)
	title := lipgloss.NewStyle().Foreground(m.theme.text).Render(truncate(oneLine(pr.Title), inner))
	// The selected card gets a heavier border as well as a colour, so the
	// selection still shows on a terminal without colour.
	border, borderColor := lipgloss.RoundedBorder(), m.theme.border
	if selected {
		border, borderColor = lipgloss.ThickBorder(), m.theme.accent
	}
	return lipgloss.NewStyle().
		Width(laneWidth).
		Padding(0, 1).
		Border(border).
		BorderForeground(borderColor).
		Render(ref + "\n" + title)
}

func (m Model) statusView() string {
	errStyle := lipgloss.NewStyle().Foreground(m.theme.errText)
	switch {
	case m.err != nil:
		return errStyle.Render("Couldn't load pull requests: " + oneLine(m.err.Error()))
	case m.storeErr != nil:
		return errStyle.Render("Couldn't load tags: " + oneLine(m.storeErr.Error()))
	case m.saveErr != nil:
		return errStyle.Render("Couldn't save lanes: " + oneLine(m.saveErr.Error()))
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
