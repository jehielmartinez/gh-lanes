package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/activity"
	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// cardPadding is the border plus one column of padding on each side.
const cardPadding = 4

const statusSeparator = " · "

// View draws the tab bar, the board or one of the card lists, the status bar
// and the help footer, with the detail modal, the tag manager or any open
// picker over them.
func (m Model) View() tea.View {
	body := lipgloss.NewStyle()
	if height := m.boardHeight(); height > 0 {
		body = body.Height(height).MaxHeight(height)
	}
	if m.width > 0 {
		body = body.MaxWidth(m.width)
	}
	var content string
	switch m.tab {
	case tabReview:
		note := ""
		if m.loaded && len(m.reviews.prs) == 0 {
			note = "No review requests."
		}
		content = m.listView(m.reviews, note)
	case tabArchived:
		content = m.archivedView()
	default:
		content = m.boardView()
	}
	screen := lipgloss.JoinVertical(lipgloss.Left, m.tabsView(), body.Render(content), m.footerView())
	if m.detail != nil {
		screen = m.overlay(screen, m.detailView())
	}
	if m.tagManager != nil {
		screen = m.overlay(screen, m.tagManager.view(m.theme, m.help, m.lanes))
	}
	if m.picker != nil {
		screen = m.overlay(screen, m.picker.view(m.theme, m.help))
	}
	if m.mergeDialog != nil {
		screen = m.overlay(screen, m.mergeDialogView())
	}
	if m.confirm != nil {
		screen = m.overlay(screen, m.confirmView())
	}
	v := tea.NewView(screen)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// footerView is the status bar over the help footer, which lists the keys of
// the detail modal while it is open, or else of the tab in view.
func (m Model) footerView() string {
	actionKeys := m.actionKeys()
	var keys help.KeyMap = actionKeys
	switch {
	case m.detail != nil:
		keys = modalHelp{keys: actionKeys}
	case m.tab != tabBoard:
		keys = listHelp{keys: actionKeys, archived: m.tab == tabArchived}
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.statusView(), m.help.View(keys))
}

// overlay draws fg centred on the terminal over bg.
func (m Model) overlay(bg, fg string) string {
	termW, termH := m.width, m.height
	if termW == 0 || termH == 0 {
		termW, termH = lipgloss.Width(bg), lipgloss.Height(bg)
	}
	x, y := centred(termW, termH, lipgloss.Width(fg), lipgloss.Height(fg))
	canvas := lipgloss.NewCanvas(max(termW, lipgloss.Width(bg)), max(termH, lipgloss.Height(bg)))
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(bg),
		lipgloss.NewLayer(fg).X(x).Y(y).Z(1),
	))
	return canvas.Render()
}

// centred is the top-left cell of a w by h box centred in an outer one.
func centred(outerW, outerH, w, h int) (x, y int) {
	return max(0, (outerW-w)/2), max(0, (outerH-h)/2)
}

// boardView draws the lanes in view, each scrolled to its own offset.
func (m Model) boardView() string {
	last := min(len(m.lanes), m.firstLane+m.lanesInView())
	cols := make([]string, 0, last-m.firstLane)
	for i := m.firstLane; i < last; i++ {
		cols = append(cols, m.laneView(m.lanes[i], i == m.focus, m.cursors[i], m.offsets[i]))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// laneHeader is a lane's name and card count.
func (m Model) laneHeader(lane board.Lane, focused bool) string {
	name := lipgloss.NewStyle().Bold(true).Underline(focused).Foreground(laneColor(m.theme, lane)).Render(lane.Tag.Name)
	return name + " " + lipgloss.NewStyle().Foreground(m.theme.muted).Render(fmt.Sprint(len(lane.PullRequests)))
}

// laneView draws a lane's header and the cards in view from offset.
func (m Model) laneView(lane board.Lane, focused bool, cursor, offset int) string {
	width := m.laneWidth()
	rows := []string{m.laneHeader(lane, focused)}
	if lane.Untagged() && m.loaded && !board.HasOpen(m.lanes) {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.muted).Render("No open pull requests."))
	}
	last := min(len(lane.PullRequests), offset+m.cardsInView())
	for i := offset; i < last; i++ {
		rows = append(rows, m.cardView(lane.PullRequests[i], width, focused && i == cursor, lane.Dimmed(lane.PullRequests[i])))
	}
	return lipgloss.NewStyle().Width(width).MarginRight(laneGap).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func laneColor(t theme, lane board.Lane) color.Color {
	if lane.Tag.Color == "" {
		return t.untagged
	}
	return lipgloss.Color(lane.Tag.Color)
}

func (m Model) cardView(pr domain.PullRequest, width int, selected, dimmed bool) string {
	inner := width - cardPadding
	number := fmt.Sprintf("#%d", pr.Number)
	marker := m.markerView(m.markers[pr.ID])
	room := inner - lipgloss.Width(number)
	if marker != "" {
		room -= lipgloss.Width(marker) + 1
	}
	ref := truncate(pr.Repository.NameWithOwner, room) + number
	ref = lipgloss.NewStyle().Foreground(m.theme.muted).Faint(dimmed).Render(ref)
	if marker != "" {
		ref += strings.Repeat(" ", max(inner-lipgloss.Width(ref)-lipgloss.Width(marker), 1)) + marker
	}
	titleColor := m.theme.text
	if m.err != nil || dimmed {
		titleColor = m.theme.muted
	}
	title := lipgloss.NewStyle().Foreground(titleColor).Faint(dimmed).Render(truncate(oneLine(pr.Title), inner))
	// The selected card gets a heavier border as well as a colour, so the
	// selection still shows on a terminal without colour.
	border, borderColor := lipgloss.RoundedBorder(), m.theme.border
	if selected {
		border, borderColor = lipgloss.ThickBorder(), m.theme.accent
	}
	status := m.cardStatusLine(pr, inner)
	if pr.Finished() {
		status = m.cardStateLine(pr, inner, dimmed)
	}
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Border(border).
		BorderForeground(borderColor).
		Render(ref + "\n" + title + "\n" + status)
}

// cardStateLine is the third line of a merged or closed card: its state badge
// in place of checks, reviews and mergeability, which no longer apply.
func (m Model) cardStateLine(pr domain.PullRequest, width int, dimmed bool) string {
	label, c := "Closed", m.theme.failure
	if pr.State == domain.StateMerged {
		label, c = "Merged", m.theme.merged
	}
	if dimmed {
		c = m.theme.muted
	}
	badge := lipgloss.NewStyle().Foreground(c).Bold(!dimmed).Faint(dimmed).Render(label)
	age := lipgloss.NewStyle().Foreground(m.theme.muted).Faint(dimmed).Render(shortAge(m.now.Sub(pr.UpdatedAt)))
	gap := max(width-lipgloss.Width(badge)-lipgloss.Width(age), 1)
	return badge + strings.Repeat(" ", gap) + age
}

// markerView is the activity marker drawn at the end of a card's first line.
func (m Model) markerView(marker activity.Marker) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(m.theme.accent)
	switch marker {
	case activity.Changed:
		return style.Render("●")
	case activity.New:
		return style.Render("new")
	}
	return ""
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
	if t := m.toastView(); t != "" {
		parts = append(parts, t)
	}
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
	if m.loadingLinks() {
		parts = append(parts, m.spinner.View()+" "+muted.Render("Loading links…"))
	}
	if line := m.linkLine(); line != "" {
		parts = append(parts, line)
	}
	changed, added := activity.Tally(m.markers)
	if changed > 0 {
		parts = append(parts, plural(changed, "PR")+" changed")
	}
	if added > 0 {
		parts = append(parts, plural(added, "new PR"))
	}
	if m.storeErr != nil {
		parts = append(parts, errStyle.Render("Couldn't load tags: "+oneLine(m.storeErr.Error())))
	}
	if m.saveErr != nil {
		parts = append(parts, errStyle.Render("Couldn't save lanes: "+oneLine(m.saveErr.Error())))
	}
	if m.tagSaveErr != nil {
		parts = append(parts, errStyle.Render("Couldn't save tags: "+oneLine(m.tagSaveErr.Error())))
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
