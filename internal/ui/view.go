package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

const (
	laneWidth = 36
	// laneGap is the margin to the right of each lane.
	laneGap = 1
	// cardPadding is the border plus one column of padding on each side.
	cardPadding = 4
	footerLines = 2
)

const statusSeparator = " · "

// View draws the board, the status bar and the help footer, with the detail
// modal or any open picker over them.
func (m Model) View() tea.View {
	body := m.boardView()
	if m.height > footerLines {
		body = lipgloss.NewStyle().Height(m.height - footerLines).MaxHeight(m.height - footerLines).Render(body)
	}
	var keys help.KeyMap = m.keys
	if m.detail != nil {
		keys = modalHelp{keys: m.keys}
	}
	screen := lipgloss.JoinVertical(lipgloss.Left, body, m.statusView(), m.help.View(keys))
	if m.detail != nil {
		screen = m.overlay(screen, m.detailView())
	}
	if m.picker != nil {
		screen = m.overlay(screen, m.picker.view(m.theme, m.help))
	}
	v := tea.NewView(screen)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
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

// cardAt finds the card drawn at a screen cell, measuring the cards as
// boardView lays them out.
func (m Model) cardAt(x, y int) (domain.PullRequest, bool) {
	if m.height > footerLines && y >= m.height-footerLines {
		return domain.PullRequest{}, false
	}
	lane := x / (laneWidth + laneGap)
	if x < 0 || lane >= len(m.lanes) || x%(laneWidth+laneGap) >= laneWidth {
		return domain.PullRequest{}, false
	}
	focused := lane == m.focus
	top := lipgloss.Height(m.laneHeader(m.lanes[lane], focused))
	for i, pr := range m.lanes[lane].PullRequests {
		bottom := top + lipgloss.Height(m.cardView(pr, focused && i == m.cursors[lane]))
		if y >= top && y < bottom {
			return pr, true
		}
		top = bottom
	}
	return domain.PullRequest{}, false
}

func (m Model) boardView() string {
	cols := make([]string, 0, len(m.lanes))
	for i, lane := range m.lanes {
		cols = append(cols, m.laneView(lane, i == m.focus, m.cursors[i]))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) laneHeader(lane board.Lane, focused bool) string {
	name := lipgloss.NewStyle().Bold(true).Underline(focused).Foreground(m.laneColor(lane)).Render(lane.Tag.Name)
	return name + " " + lipgloss.NewStyle().Foreground(m.theme.muted).Render(fmt.Sprint(len(lane.PullRequests)))
}

func (m Model) laneView(lane board.Lane, focused bool, cursor int) string {
	rows := []string{m.laneHeader(lane, focused)}
	if lane.Untagged() && m.loaded && len(m.prs) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.muted).Render("No open pull requests."))
	}
	for i, pr := range lane.PullRequests {
		rows = append(rows, m.cardView(pr, focused && i == cursor))
	}
	return lipgloss.NewStyle().Width(laneWidth).MarginRight(laneGap).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
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
	ref := truncate(pr.Repository.NameWithOwner, inner-lipgloss.Width(number)) + number
	ref = lipgloss.NewStyle().Foreground(m.theme.muted).Render(ref)
	titleColor := m.theme.text
	if m.err != nil {
		titleColor = m.theme.muted
	}
	title := lipgloss.NewStyle().Foreground(titleColor).Render(truncate(oneLine(pr.Title), inner))
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
		Render(ref + "\n" + title + "\n" + m.cardStatusLine(pr, inner))
}

// cardStatusLine is the card's third line: checks, review decision and
// mergeability on the left, age on the right.
func (m Model) cardStatusLine(pr domain.PullRequest, width int) string {
	var parts []string
	style := func(c color.Color, s string) string {
		return lipgloss.NewStyle().Foreground(c).Render(s)
	}
	counts := domain.CountChecks(pr.Checks)
	if counts.Passed > 0 {
		parts = append(parts, style(m.theme.success, fmt.Sprintf("✓%d", counts.Passed)))
	}
	if counts.Failed > 0 {
		parts = append(parts, style(m.theme.failure, fmt.Sprintf("✗%d", counts.Failed)))
	}
	if counts.Pending > 0 {
		parts = append(parts, style(m.theme.pending, fmt.Sprintf("●%d", counts.Pending)))
	}
	switch pr.ReviewDecision {
	case domain.ReviewApproved:
		parts = append(parts, style(m.theme.success, "approved"))
	case domain.ReviewChangesRequested:
		parts = append(parts, style(m.theme.failure, "changes req."))
	case domain.ReviewRequired:
		parts = append(parts, style(m.theme.muted, "needs review"))
	}
	switch pr.MergeStatus() {
	case domain.MergeStatusConflict:
		parts = append(parts, style(m.theme.failure, "⚠ conflict"))
	case domain.MergeStatusBehind:
		parts = append(parts, style(m.theme.pending, "↓ behind"))
	case domain.MergeStatusChecking:
		parts = append(parts, style(m.theme.muted, "checking…"))
	}

	age := style(m.theme.muted, shortAge(m.now.Sub(pr.UpdatedAt)))
	left := lipgloss.NewStyle().MaxWidth(width - lipgloss.Width(age) - 1).Render(strings.Join(parts, " "))
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(age), 1)
	return left + strings.Repeat(" ", gap) + age
}

func (m Model) statusView() string {
	muted := lipgloss.NewStyle().Foreground(m.theme.muted)
	errStyle := lipgloss.NewStyle().Foreground(m.theme.errText)
	var parts []string
	switch {
	case !m.loaded && m.err != nil:
		parts = append(parts, errStyle.Render("Couldn't load pull requests: "+oneLine(m.err.Error())))
	case !m.loaded:
		parts = append(parts, m.spinner.View()+" "+muted.Render("Loading pull requests…"))
	default:
		if m.err != nil {
			parts = append(parts, errStyle.Render("stale"), errStyle.Render("Couldn't refresh: "+oneLine(m.err.Error())))
		}
		updated := muted.Render("updated " + ago(m.now.Sub(m.updatedAt)))
		if m.refreshing {
			updated = m.spinner.View() + " " + updated
		}
		parts = append(parts, updated)
	}
	if m.storeErr != nil {
		parts = append(parts, errStyle.Render("Couldn't load tags: "+oneLine(m.storeErr.Error())))
	}
	if m.saveErr != nil {
		parts = append(parts, errStyle.Render("Couldn't save lanes: "+oneLine(m.saveErr.Error())))
	}
	if m.rateLimit.Low() {
		parts = append(parts, errStyle.Render(fmt.Sprintf("rate limit %d/%d left", m.rateLimit.Remaining, m.rateLimit.Limit)))
	}
	if m.configErr != nil {
		parts = append(parts, errStyle.Render(oneLine(m.configErr.Error())))
	}
	return strings.Join(parts, muted.Render(statusSeparator))
}

// ago says how long ago something happened, to the second under a minute.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", max(int(d/time.Second), 0))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	}
	return fmt.Sprintf("%dh ago", int(d/time.Hour))
}

// shortAge is a card's age since its last update, in the largest whole unit.
func shortAge(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < day:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 30*day:
		return fmt.Sprintf("%dd", int(d/day))
	case d < 365*day:
		return fmt.Sprintf("%dmo", int(d/(30*day)))
	}
	return fmt.Sprintf("%dy", int(d/(365*day)))
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
