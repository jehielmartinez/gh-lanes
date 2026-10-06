package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// theme holds every colour the views use, resolved for the terminal's
// background.
type theme struct {
	untagged color.Color
	border   color.Color
	muted    color.Color
	text     color.Color
	errText  color.Color
	success  color.Color
	failure  color.Color
	pending  color.Color
	merged   color.Color
}

func newTheme(dark bool) theme {
	pick := lipgloss.LightDark(dark)
	return theme{
		untagged: pick(lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF")),
		border:   pick(lipgloss.Color("#D1D5DB"), lipgloss.Color("#4B5563")),
		muted:    pick(lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF")),
		text:     pick(lipgloss.Color("#111827"), lipgloss.Color("#F3F4F6")),
		errText:  pick(lipgloss.Color("#B91C1C"), lipgloss.Color("#F87171")),
		success:  pick(lipgloss.Color("#15803D"), lipgloss.Color("#4ADE80")),
		failure:  pick(lipgloss.Color("#B91C1C"), lipgloss.Color("#F87171")),
		pending:  pick(lipgloss.Color("#B45309"), lipgloss.Color("#FBBF24")),
		merged:   pick(lipgloss.Color("#7E22CE"), lipgloss.Color("#C084FC")),
	}
}
