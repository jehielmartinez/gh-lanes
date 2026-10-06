package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// modalScale is the share of the screen the detail modal covers, in tenths.
const modalScale = 9

// modalFrame is the modal's border plus one column of padding on each side.
const modalFrame = 4

// detail is the open detail modal.
type detail struct {
	pr domain.PullRequest
	// fresh is whether pr came from the detail query rather than the board.
	fresh    bool
	fetching bool
	// seq identifies the fetch in flight, so a reply meant for an earlier
	// opening of the modal is dropped.
	seq int
	// fetchedAt is when the last fetch finished, successful or not; the next
	// one is due an interval after it.
	fetchedAt time.Time
	err       error
	viewport  viewport.Model
}

type detailMsg struct {
	seq       int
	pr        domain.PullRequest
	rateLimit domain.RateLimit
	err       error
	at        time.Time
}

// openDetail opens the modal on a pull request, showing what the board knows
// until the fresh copy arrives.
func (m Model) openDetail(pr domain.PullRequest) (Model, tea.Cmd) {
	vp := viewport.New()
	vp.KeyMap = m.keys.Modal.viewportKeys()
	m.detail = &detail{pr: pr, viewport: vp}
	return m.fetchDetail()
}

func (m Model) closeDetail() Model {
	m.detail = nil
	return m
}

// fetchDetail starts a fetch of the open pull request unless one is in flight.
func (m Model) fetchDetail() (Model, tea.Cmd) {
	if m.detail == nil || m.detail.fetching {
		return m, nil
	}
	m.detailSeq++
	d := *m.detail
	d.fetching, d.seq = true, m.detailSeq
	m.detail = &d
	gh, now, seq, id := m.opts.GitHub, m.opts.Now, d.seq, d.pr.ID
	fetch := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		pr, limit, err := gh.PullRequest(ctx, id)
		return detailMsg{seq: seq, pr: pr, rateLimit: limit, err: err, at: now()}
	}
	return m, tea.Batch(fetch, m.spinner.Tick)
}

func (m Model) detailFetched(msg detailMsg) Model {
	if m.detail == nil || msg.seq != m.detail.seq {
		return m
	}
	d := *m.detail
	d.fetching = false
	d.fetchedAt = msg.at
	d.err = msg.err
	if msg.err == nil {
		d.pr, d.fresh = msg.pr, true
	}
	if msg.rateLimit != (domain.RateLimit{}) {
		m.rateLimit = msg.rateLimit
	}
	m.detail = &d
	return m
}

// detailDue starts the modal's own refresh once an interval has passed since
// its last fetch.
func (m Model) detailDue() (Model, tea.Cmd) {
	if m.detail == nil || m.now.Sub(m.detail.fetchedAt) < m.refreshInterval {
		return m, nil
	}
	return m.fetchDetail()
}

func (m Model) detailKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	d := *m.detail
	switch {
	case key.Matches(msg, m.keys.Modal.Close):
		return m.closeDetail(), nil
	case key.Matches(msg, m.keys.Modal.Top):
		d.viewport.GotoTop()
	case key.Matches(msg, m.keys.Modal.Bottom):
		d.viewport.GotoBottom()
	default:
		d.viewport, _ = d.viewport.Update(msg)
	}
	m.detail = &d
	return m, nil
}

func (m Model) detailScroll(msg tea.MouseWheelMsg) Model {
	d := *m.detail
	d.viewport, _ = d.viewport.Update(msg)
	m.detail = &d
	return m
}

// modalSize is the modal's outer width and height for the current screen.
func (m Model) modalSize() (width, height int) {
	return max(m.width*modalScale/10, modalFrame+10), max(m.height*modalScale/10, 5)
}

// syncDetail lays the modal's content into its viewport, so scrolling is
// measured against what is drawn.
func (m Model) syncDetail() Model {
	if m.detail == nil {
		return m
	}
	width, height := m.modalSize()
	inner := width - modalFrame
	d := *m.detail
	d.viewport.SetWidth(inner)
	d.viewport.SetHeight(max(height-2-lipgloss.Height(m.detailStatus(inner)), 1))
	d.viewport.SetContent(m.detailContent(inner))
	m.detail = &d
	return m
}

// detailView is the modal box, sized and filled for the current screen.
func (m Model) detailView() string {
	width, height := m.modalSize()
	inner := width - modalFrame
	body := lipgloss.JoinVertical(lipgloss.Left, m.detail.viewport.View(), m.detailStatus(inner))
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.border).
		Render(body)
}

func (m Model) detailStatus(width int) string {
	d := m.detail
	muted := lipgloss.NewStyle().Foreground(m.theme.muted)
	errStyle := lipgloss.NewStyle().Foreground(m.theme.errText)
	var line string
	switch {
	case !d.fresh && d.err != nil:
		line = errStyle.Render("Couldn't load this pull request: " + oneLine(d.err.Error()))
	case !d.fresh:
		line = m.spinner.View() + " " + muted.Render("Loading…")
	case d.err != nil:
		line = errStyle.Render("stale") + muted.Render(statusSeparator) + errStyle.Render("Couldn't refresh: "+oneLine(d.err.Error()))
	default:
		line = muted.Render("updated " + ago(m.now.Sub(d.fetchedAt)))
		if d.fetching {
			line = m.spinner.View() + " " + line
		}
	}
	return lipgloss.NewStyle().Width(width).Render(line)
}

// detailContent is everything the modal scrolls through.
func (m Model) detailContent(width int) string {
	pr := m.detail.pr
	muted := lipgloss.NewStyle().Foreground(m.theme.muted)
	heading := lipgloss.NewStyle().Bold(true).Foreground(m.theme.text)
	wrap := lipgloss.NewStyle().Width(width)

	ref := fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
	author := pr.Author
	if author == "" {
		author = "ghost"
	}
	lines := []string{
		muted.Render(ref) + "  " + m.stateBadge(pr),
		wrap.Bold(true).Foreground(m.theme.text).Render(oneLine(pr.Title)),
		wrap.Render(muted.Render(author + statusSeparator + pr.HeadRef + " → " + pr.BaseRef)),
		wrap.Render(muted.Render("Created " + m.timestamp(pr.CreatedAt) + statusSeparator + "Updated " + m.timestamp(pr.UpdatedAt))),
		"",
		heading.Render("Merge status"),
		wrap.Render("  " + m.mergeLine(pr)),
		"",
		heading.Render("Review"),
		"  " + m.reviewLine(pr.ReviewDecision),
		"",
		heading.Render("Auto-merge"),
		wrap.Render("  " + m.autoMergeLine(pr.AutoMerge)),
		"",
		heading.Render("Checks") + "  " + muted.Render(checkSummary(pr.Checks)),
	}
	lines = append(lines, m.checkLines(pr.Checks, width)...)
	return strings.Join(lines, "\n")
}

func (m Model) stateBadge(pr domain.PullRequest) string {
	label, c := "Open", m.theme.success
	switch {
	case pr.State == domain.StateMerged:
		label, c = "Merged", m.theme.merged
	case pr.State == domain.StateClosed:
		label, c = "Closed", m.theme.failure
	case pr.IsDraft:
		label, c = "Draft", m.theme.muted
	}
	return lipgloss.NewStyle().Bold(true).Foreground(c).Render("[" + label + "]")
}

func (m Model) mergeLine(pr domain.PullRequest) string {
	icon, c := "✓", m.theme.success
	switch pr.MergeStatus() {
	case domain.MergeStatusConflict:
		icon, c = "⚠", m.theme.failure
	case domain.MergeStatusBehind:
		icon, c = "↓", m.theme.pending
	case domain.MergeStatusChecking:
		icon, c = "…", m.theme.muted
	}
	if pr.State != domain.StateOpen {
		icon, c = "•", m.theme.muted
	} else if pr.MergeStateStatus == domain.MergeStateBlocked || pr.MergeStateStatus == domain.MergeStateDraft {
		icon, c = "●", m.theme.pending
	}
	return lipgloss.NewStyle().Foreground(c).Render(icon) + " " + pr.MergeSentence()
}

func (m Model) reviewLine(decision domain.ReviewDecision) string {
	style := func(c color.Color, s string) string { return lipgloss.NewStyle().Foreground(c).Render(s) }
	switch decision {
	case domain.ReviewApproved:
		return style(m.theme.success, "✓") + " Approved"
	case domain.ReviewChangesRequested:
		return style(m.theme.failure, "✗") + " Changes requested"
	case domain.ReviewRequired:
		return style(m.theme.pending, "●") + " Review required"
	}
	return style(m.theme.muted, "No review required")
}

func (m Model) autoMergeLine(am *domain.AutoMerge) string {
	if am == nil {
		return lipgloss.NewStyle().Foreground(m.theme.muted).Render("Off")
	}
	method := map[domain.MergeMethod]string{
		domain.MergeMethodMerge:  "Merge commit",
		domain.MergeMethodSquash: "Squash",
		domain.MergeMethodRebase: "Rebase",
	}[am.Method]
	if method == "" {
		method = "Merge"
	}
	line := method + " when ready"
	if am.EnabledBy != "" {
		line += ", enabled by " + am.EnabledBy
	}
	if !am.EnabledAt.IsZero() {
		line += " on " + m.timestamp(am.EnabledAt)
	}
	return line
}

// checkSummary is the one-line tally of the checks, like
// "12 passed · 1 failed · 2 running".
func checkSummary(checks []domain.Check) string {
	if len(checks) == 0 {
		return "No checks"
	}
	c := domain.CountChecks(checks)
	var parts []string
	for _, p := range []struct {
		n     int
		label string
	}{{c.Passed, "passed"}, {c.Failed, "failed"}, {c.Pending, "running"}, {c.Skipped, "skipped"}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.label))
		}
	}
	return strings.Join(parts, statusSeparator)
}

func (m Model) checkLines(checks []domain.Check, width int) []string {
	groups := domain.GroupChecks(checks)
	nameWidth := 0
	for _, ch := range checks {
		nameWidth = max(nameWidth, lipgloss.Width(oneLine(ch.Name)))
	}
	nameWidth = min(nameWidth, max(width/2, 10))
	muted := lipgloss.NewStyle().Foreground(m.theme.muted)
	var lines []string
	for _, g := range groups {
		lines = append(lines, "  "+lipgloss.NewStyle().Bold(true).Render(groupLabel(g)))
		for _, ch := range g.Checks {
			name := truncate(oneLine(ch.Name), nameWidth)
			name += strings.Repeat(" ", nameWidth-lipgloss.Width(name))
			line := "    " + m.checkIcon(ch.Outcome) + " " + name + "  " + muted.Render(checkTiming(ch, m.now))
			lines = append(lines, lipgloss.NewStyle().MaxWidth(width).Render(line))
		}
	}
	return lines
}

func groupLabel(g domain.CheckGroup) string {
	switch {
	case g.Kind == domain.CommitStatus:
		return "Commit statuses"
	case g.Workflow == "":
		return "Other checks"
	}
	return oneLine(g.Workflow)
}

func (m Model) checkIcon(o domain.CheckOutcome) string {
	icon, c := "–", m.theme.muted
	switch o {
	case domain.CheckPassed:
		icon, c = "✓", m.theme.success
	case domain.CheckFailed:
		icon, c = "✗", m.theme.failure
	case domain.CheckPending:
		icon, c = "●", m.theme.pending
	}
	return lipgloss.NewStyle().Foreground(c).Render(icon)
}

// checkTiming is how long a check ran, or how long ago a commit status was
// set.
func checkTiming(ch domain.Check, now time.Time) string {
	switch {
	case ch.Kind == domain.CommitStatus:
		if ch.StartedAt.IsZero() {
			return ""
		}
		return relative(now.Sub(ch.StartedAt))
	case ch.Outcome == domain.CheckSkipped:
		return "skipped"
	case ch.Outcome == domain.CheckPending && ch.StartedAt.IsZero():
		return "queued"
	case ch.Outcome == domain.CheckPending:
		return "running " + duration(now.Sub(ch.StartedAt))
	case !ch.StartedAt.IsZero() && !ch.CompletedAt.IsZero():
		return duration(ch.CompletedAt.Sub(ch.StartedAt))
	case !ch.CompletedAt.IsZero():
		return relative(now.Sub(ch.CompletedAt))
	}
	return ""
}

// timestamp is a time in the clock's timezone with its age, like
// "Mar 5, 2026 09:00 (3h ago)".
func (m Model) timestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.In(m.now.Location()).Format("Jan 2, 2006 15:04") + " (" + relative(m.now.Sub(t)) + ")"
}

func relative(d time.Duration) string {
	age := shortAge(d)
	if age == "now" {
		return "just now"
	}
	return age + " ago"
}

// duration is how long something took, to the second under an hour.
func duration(d time.Duration) string {
	d = max(d, 0)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d/time.Minute), int(d%time.Minute/time.Second))
	}
	return fmt.Sprintf("%dh %dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
}
