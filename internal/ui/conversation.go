package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/markdown"
)

// indentStep is how far each level of the conversation is indented.
const indentStep = 2

// descriptionLines is the modal's Description section.
func (m Model) descriptionLines(conv domain.Conversation, width int) []string {
	lines := []string{m.sectionHeading("Description")}
	if strings.TrimSpace(conv.Body) == "" {
		return append(lines, indented(1, m.muted("No description provided.")))
	}
	return append(lines, m.markdownLines(conv.Body, 1, width)...)
}

// conversationLines is the modal's Conversation section: one timeline of
// comments, reviews and review threads, oldest first.
func (m Model) conversationLines(conv domain.Conversation, expandResolved bool, width int) []string {
	timeline := conv.Timeline()
	lines := []string{m.sectionHeading("Conversation") + "  " + m.muted(strconv.Itoa(len(timeline)))}
	if len(timeline) == 0 {
		return append(lines, indented(1, m.muted("No comments yet.")))
	}
	for i, e := range timeline {
		if i > 0 {
			lines = append(lines, "")
		}
		switch {
		case e.Comment != nil:
			lines = append(lines, m.commentLines(*e.Comment, "commented", 1, width)...)
		case e.Review != nil:
			lines = append(lines, m.reviewLines(*e.Review, width)...)
		case e.Thread != nil:
			lines = append(lines, m.threadLines(*e.Thread, expandResolved, width)...)
		}
	}
	return lines
}

func (m Model) commentLines(c domain.Comment, verb string, depth, width int) []string {
	header := m.bold(authorName(c.Author))
	if verb != "" {
		header += " " + verb
	}
	header += m.muted(statusSeparator + m.timestamp(c.CreatedAt))
	lines := []string{indentedWrap(depth, header, width)}
	return append(lines, m.markdownLines(c.Body, depth+1, width)...)
}

func (m Model) reviewLines(r domain.Review, width int) []string {
	header := m.bold(authorName(r.Author)) + " " + m.reviewBadge(r.State) +
		m.muted(statusSeparator+m.timestamp(r.SubmittedAt))
	lines := []string{indentedWrap(1, header, width)}
	return append(lines, m.markdownLines(r.Body, 2, width)...)
}

func (m Model) reviewBadge(state string) string {
	label, c := "Commented", m.theme.muted
	switch state {
	case domain.ReviewStateApproved:
		label, c = "Approved", m.theme.success
	case domain.ReviewStateChangesRequested:
		label, c = "Changes requested", m.theme.failure
	case domain.ReviewStateDismissed:
		label = "Dismissed"
	}
	return lipgloss.NewStyle().Bold(true).Foreground(c).Render("[" + label + "]")
}

// threadLines draws a review thread: where it is, whether it is resolved, and
// its comments beneath, unless it is resolved and resolved threads are
// collapsed.
func (m Model) threadLines(t domain.Thread, expandResolved bool, width int) []string {
	collapsed := t.Resolved && !expandResolved
	marker, state := "▾", lipgloss.NewStyle().Foreground(m.theme.pending).Render("Unresolved")
	if t.Resolved {
		state = lipgloss.NewStyle().Foreground(m.theme.success).Render("Resolved")
	}
	if collapsed {
		marker = "▸"
	}
	header := m.muted(marker) + " " + m.bold(threadLocation(t)) + m.muted(statusSeparator) + state
	if collapsed {
		header += m.muted(statusSeparator + plural(len(t.Comments), "comment"))
	} else {
		header += m.muted(statusSeparator + m.timestamp(t.StartedAt()))
	}
	lines := []string{indentedWrap(1, header, width)}
	if collapsed {
		return lines
	}
	for _, c := range t.Comments {
		lines = append(lines, m.commentLines(c, "", 2, width)...)
	}
	return lines
}

// threadLocation is a thread's file:line, or just the file for a comment on
// the whole file.
func threadLocation(t domain.Thread) string {
	path := oneLine(t.Path)
	if t.Line == 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, t.Line)
}

// markdownLines renders markdown at an indent depth, with links drawn
// underlined in the accent colour.
func (m Model) markdownLines(src string, depth, width int) []string {
	indent := strings.Repeat(" ", depth*indentStep)
	var lines []string
	for _, l := range markdown.Render(src, width-len(indent)) {
		var b strings.Builder
		b.WriteString(indent)
		for _, sp := range l {
			b.WriteString(m.spanStyle(sp).Render(sp.Text))
		}
		lines = append(lines, b.String())
	}
	return lines
}

func (m Model) spanStyle(sp markdown.Span) lipgloss.Style {
	s := lipgloss.NewStyle()
	if sp.Style&markdown.Bold != 0 {
		s = s.Bold(true)
	}
	if sp.Style&markdown.Italic != 0 {
		s = s.Italic(true)
	}
	if sp.Style&markdown.Strike != 0 {
		s = s.Strikethrough(true)
	}
	if sp.Style&markdown.Heading != 0 {
		s = s.Foreground(m.theme.text)
	}
	if sp.Style&(markdown.Code|markdown.Muted) != 0 {
		s = s.Foreground(m.theme.muted)
	}
	if sp.URL != "" {
		s = s.Underline(true).Foreground(m.theme.accent)
	}
	return s
}

func (m Model) sectionHeading(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(m.theme.text).Render(s)
}

func (m Model) muted(s string) string {
	return lipgloss.NewStyle().Foreground(m.theme.muted).Render(s)
}

func (m Model) bold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

func indented(depth int, s string) string {
	return strings.Repeat(" ", depth*indentStep) + s
}

// indentedWrap wraps s to fit beside its indent, keeping every wrapped line
// indented.
func indentedWrap(depth int, s string, width int) string {
	indent := strings.Repeat(" ", depth*indentStep)
	wrapped := lipgloss.NewStyle().Width(max(width-len(indent), 1)).Render(s)
	return indent + strings.ReplaceAll(wrapped, "\n", "\n"+indent)
}

// authorName is a login, or "ghost" for a deleted account, as GitHub shows it.
func authorName(login string) string {
	if login == "" {
		return "ghost"
	}
	return oneLine(login)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
