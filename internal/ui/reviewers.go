package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// rollCallLines are the Status section's rows of reviewers, under the review
// decision. With nobody to list they name only the stuck case: a review the
// base branch requires that nobody was asked for.
func (m Model) rollCallLines(pr domain.PullRequest, width int) page {
	rows := pr.RollCall()
	if len(rows) == 0 {
		if pr.ReviewDecision == domain.ReviewRequired {
			return text(indented(2, m.muted("No reviewers requested")))
		}
		return page{}
	}
	nameWidth := 0
	for _, r := range rows {
		nameWidth = max(nameWidth, lipgloss.Width(reviewerName(r)))
	}
	nameWidth = min(nameWidth, max(width/2, 10))
	verdictWidth := lipgloss.Width(verdictLabel(domain.ReviewStateChangesRequested))
	var p page
	for _, r := range rows {
		name := truncate(reviewerName(r), nameWidth)
		label, age := verdictLabel(r.Verdict), relative(m.now.Sub(r.VerdictAt))
		if r.Awaiting {
			label, age = "awaiting review", ""
			if r.Verdict != "" {
				age = "(" + verdictLabel(r.Verdict) + " " + relative(m.now.Sub(r.VerdictAt)) + ")"
			}
		}
		columns := " " + name + strings.Repeat(" ", nameWidth-lipgloss.Width(name)) +
			"  " + label + strings.Repeat(" ", verdictWidth-lipgloss.Width(label)) + "  "
		icon, c, sided := m.reviewerIcon(r)
		line := m.muted(icon + columns + age)
		if sided {
			line = lipgloss.NewStyle().Foreground(c).Render(icon) + columns + m.muted(age)
		}
		p.lines = append(p.lines, lipgloss.NewStyle().MaxWidth(width).Render(indented(2, line)))
	}
	return p
}

func reviewerName(r domain.Reviewer) string {
	switch {
	case r.Name == "":
		return "a team you can't see"
	case r.IsViewer:
		return "you"
	case r.Team:
		return "@" + oneLine(r.Name)
	}
	return oneLine(r.Name)
}

func verdictLabel(state string) string {
	switch state {
	case domain.ReviewStateApproved:
		return "approved"
	case domain.ReviewStateChangesRequested:
		return "changes requested"
	case domain.ReviewStateDismissed:
		return "dismissed"
	}
	return "commented"
}

// reviewerIcon is a row's glyph and colour. sided is false for a reviewer
// whose verdict takes no side, whose whole row is muted.
func (m Model) reviewerIcon(r domain.Reviewer) (icon string, c color.Color, sided bool) {
	switch {
	case r.Awaiting:
		return "●", m.theme.pending, true
	case r.Verdict == domain.ReviewStateChangesRequested:
		return "✗", m.theme.failure, true
	case r.Verdict == domain.ReviewStateApproved:
		return "✓", m.theme.success, true
	case r.Verdict == domain.ReviewStateDismissed:
		return "–", m.theme.muted, false
	}
	return "○", m.theme.muted, false
}
