package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// rollCallLines are the Status section's rows of reviewers, under the review
// decision.
func (m Model) rollCallLines(pr domain.PullRequest, width int) page {
	if pr.NobodyAskedToReview() {
		return text(indented(2, m.muted("No reviewers requested")))
	}
	rows := pr.RollCall()
	nameWidth := 0
	for _, r := range rows {
		nameWidth = max(nameWidth, lipgloss.Width(reviewerName(r.Identity)))
	}
	nameWidth = min(nameWidth, max(width/2, 10))
	labelWidth, _, _ := m.verdictLook(domain.ReviewStateChangesRequested)
	var p page
	for _, r := range rows {
		label, glyph, c := m.verdictLook(r.Verdict)
		age := relative(m.now.Sub(r.VerdictAt))
		if r.Awaiting {
			if r.Verdict != "" {
				age = "(" + label + " " + age + ")"
			} else {
				age = ""
			}
			label, glyph, c = "awaiting review", "●", m.theme.pending
		}
		columns := " " + padRight(truncate(reviewerName(r.Identity), nameWidth), nameWidth) +
			"  " + padRight(label, lipgloss.Width(labelWidth)) + "  "
		line := lipgloss.NewStyle().Foreground(c).Render(glyph) + columns + m.muted(age)
		// A reviewer GitHub won't name, or one whose verdict takes no side,
		// recedes.
		if r.Name == "" || (!r.Awaiting && !r.Verdict.TakesSide()) {
			line = m.muted(glyph + columns + age)
		}
		p.lines = append(p.lines, lipgloss.NewStyle().MaxWidth(width).Render(indented(2, line)))
	}
	return p
}

// reviewerName is how the roll-call names a reviewer. GitHub returns no name
// for a team the login can't see, and also for a reviewer whose account is
// gone, so it can't say which it was.
func reviewerName(who domain.Identity) string {
	switch {
	case who.Name == "":
		return "a reviewer you can't see"
	case who.IsViewer:
		return "you"
	case who.Team:
		return "@" + oneLine(who.Name)
	}
	return oneLine(who.Name)
}

// verdictLook is how a review state reads and is marked wherever a review is
// drawn: its lower-case label, glyph and colour.
func (m Model) verdictLook(state domain.ReviewState) (label, glyph string, c color.Color) {
	switch state {
	case domain.ReviewStateApproved:
		return "approved", "✓", m.theme.success
	case domain.ReviewStateChangesRequested:
		return "changes requested", "✗", m.theme.failure
	case domain.ReviewStateDismissed:
		return "dismissed", "–", m.theme.muted
	}
	return "commented", "○", m.theme.muted
}

// padRight pads s with spaces to width cells.
func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(width-lipgloss.Width(s), 0))
}

// capitalized is s with its first letter upper-cased, for a label that starts
// a line or badge.
func capitalized(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
