package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/markdown"
)

// section is one collapsible part of the detail modal, numbered in the order
// it is drawn and toggled by the matching number key.
type section int

const (
	sectionStatus section = iota
	sectionChecks
	sectionDescription
	sectionConversation
	sectionCount
)

func (s section) title() string {
	return [...]string{"Status", "Checks", "Description", "Conversation"}[s]
}

// key is the number key that toggles s.
func (s section) key() string { return string(rune('1' + s)) }

// sectionKeys are the number keys that toggle sections, one per section.
func sectionKeys() []string {
	keys := make([]string, sectionCount)
	for s := range sectionCount {
		keys[s] = s.key()
	}
	return keys
}

// sectionKeysHelp names the section keys as a range, like "1-4".
func sectionKeysHelp() string {
	return sectionStatus.key() + "-" + (sectionCount - 1).key()
}

// sectionFromKey is the section a number key toggles.
func sectionFromKey(k string) (section, bool) {
	if len(k) != 1 || k[0] < '1' || int(k[0]-'1') >= int(sectionCount) {
		return 0, false
	}
	return section(k[0] - '1'), true
}

// sectionSet is which sections are open. An array, so copying a detail
// copies it too.
type sectionSet [sectionCount]bool

// defaultSections opens what needs attention: the status always, the checks
// only when one failed or is still running, and nothing long.
func defaultSections(pr domain.PullRequest) sectionSet {
	c := domain.CountChecks(pr.Checks)
	return sectionSet{
		sectionStatus: true,
		sectionChecks: c.Failed+c.Pending > 0,
	}
}

// sectionHeader is a section's one-line heading: a ▾ or ▸ marker, its
// title, a muted summary that stays useful while the section is closed, and
// at the right edge a hint naming the key that toggles it.
func (m Model) sectionHeader(s section, open bool, summary string, width int) string {
	marker, verb := "▸", "open"
	if open {
		marker, verb = "▾", "close"
	}
	hint := s.key() + " " + verb
	title := marker + " " + s.title()
	// The summary gives way first, so the title and hint always show.
	room := width - lipgloss.Width(title) - lipgloss.Width(hint) - 4
	if room < 0 {
		return lipgloss.NewStyle().MaxWidth(width).Render(m.muted(marker) + " " + m.sectionHeading(s.title()))
	}
	line := m.muted(marker) + " " + m.sectionHeading(s.title())
	used := lipgloss.Width(title)
	if summary != "" && room > 1 {
		summary = truncate(summary, room)
		line += "  " + m.muted(summary)
		used += 2 + lipgloss.Width(summary)
	}
	return line + strings.Repeat(" ", width-used-lipgloss.Width(hint)) + m.muted(hint)
}

// statusSummary is the status section's line while it is closed: whether the
// PR can merge and what its review needs.
func (m Model) statusSummary(pr domain.PullRequest) string {
	return oneLine(pr.MergeSentence()) + statusSeparator + plainReview(pr.ReviewDecision)
}

func plainReview(decision domain.ReviewDecision) string {
	switch decision {
	case domain.ReviewApproved:
		return "Approved"
	case domain.ReviewChangesRequested:
		return "Changes requested"
	case domain.ReviewRequired:
		return "Review required"
	}
	return "No review required"
}

// descriptionSummary is the description's length, in lines as rendered.
func descriptionSummary(body string, width int) string {
	if strings.TrimSpace(body) == "" {
		return "none"
	}
	return plural(len(markdown.Render(body, width)), "line")
}

// descriptionPreview is the description's first line of text, for a glance
// at what the PR is about while the section is closed.
func descriptionPreview(body string, width int) string {
	for _, l := range markdown.Render(body, width) {
		var b strings.Builder
		for _, sp := range l {
			b.WriteString(sp.Text)
		}
		if s := strings.TrimSpace(b.String()); s != "" {
			return truncate(s, width)
		}
	}
	return ""
}

// conversationSummary is the timeline's size and who spoke last.
func (m Model) conversationSummary(timeline []domain.TimelineEntry) string {
	if len(timeline) == 0 {
		return "0"
	}
	last := timeline[len(timeline)-1]
	return strconv.Itoa(len(timeline)) + statusSeparator + "latest " + authorName(entryAuthor(last)) + " " + relative(m.now.Sub(last.At))
}

func entryAuthor(e domain.TimelineEntry) string {
	switch {
	case e.Comment != nil:
		return e.Comment.Author
	case e.Review != nil:
		return e.Review.Author
	case e.Thread != nil && len(e.Thread.Comments) > 0:
		return e.Thread.Comments[0].Author
	}
	return ""
}
