package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/links"
	"github.com/jehielmartinez/gh-lanes/internal/markdown"
)

// indentStep is how far each level of the conversation is indented.
const indentStep = 2

// descriptionLines is the body of the modal's Description section.
func (m Model) descriptionLines(conv domain.Conversation, width int) page {
	if strings.TrimSpace(conv.Body) == "" {
		return text(indented(1, m.muted("No description provided.")))
	}
	return m.markdownLines(conv.Body, 1, width)
}

// conversationLines is the body of the modal's Conversation section: one
// timeline of comments, reviews and review threads, oldest first, each behind
// a bar in its own colour so one entry reads apart from the next. A review's
// threads sit under it, behind its bar.
func (m Model) conversationLines(timeline []domain.TimelineEntry, f folding, width int) page {
	if len(timeline) == 0 {
		return text(indented(1, m.muted("No comments yet.")))
	}
	inner := width - lipgloss.Width(entryGutter)
	var p page
	for i, e := range timeline {
		if i > 0 {
			p = p.then(text(""))
		}
		switch {
		case e.Comment != nil:
			p = p.then(m.gutter(m.commentLines(*e.Comment, "commented", 0, inner), m.theme.accent))
		case e.Review != nil:
			_, _, c := m.verdictLook(e.Review.State)
			p = p.then(m.gutter(m.reviewLines(*e.Review, e.Threads, f, inner), c))
		case e.Thread != nil:
			p = p.then(m.gutter(m.threadLines(*e.Thread, 0, f, inner), m.threadColor(*e.Thread)))
		}
	}
	return p
}

// entryGutter is drawn left of every line of a timeline entry: an indent,
// the entry's bar and a space.
const entryGutter = "  ┃ "

// gutter draws p behind the entry bar in colour c, moving its links with it.
func (m Model) gutter(p page, c color.Color) page {
	bar := lipgloss.NewStyle().Foreground(c).Render(entryGutter)
	lines := make([]string, len(p.lines))
	for i, l := range p.lines {
		lines[i] = bar + l
	}
	return page{lines: lines, links: p.links.Indent(lipgloss.Width(entryGutter)), folds: p.folds}
}

// threadColor marks open threads as needing attention and lets resolved ones
// recede.
func (m Model) threadColor(t domain.Thread) color.Color {
	if t.Resolved {
		return m.theme.muted
	}
	return m.theme.pending
}

func (m Model) commentLines(c domain.Comment, verb string, depth, width int) page {
	header := m.bold(authorName(c.Author))
	if verb != "" {
		header += " " + verb
	}
	header += m.muted(statusSeparator + m.timestamp(c.CreatedAt))
	return text(indentedWrap(depth, header, width)).then(m.markdownLines(c.Body, depth, width))
}

// reviewLines draws a review: its verdict, who gave it and when, then what
// they wrote and the threads they opened with it, a blank line apart. A
// review with anything beneath its heading folds, and folded it says what it
// hides.
func (m Model) reviewLines(r domain.Review, threads []*domain.Thread, f folding, width int) page {
	_, glyph, c := m.verdictLook(r.State)
	var body page
	if strings.TrimSpace(r.Body) != "" {
		body = m.markdownLines(r.Body, 1, width)
	}
	key := reviewFold(r.ID)
	folds := r.ID != "" && (len(body.lines) > 0 || len(threads) > 0)
	open := !folds || f.isOpen(key, true)

	verb := reviewVerb(r.State)
	who := m.bold(authorName(r.Author)) + " " + verb
	if r.State.TakesSide() {
		who = m.bold(authorName(r.Author)) + " " + lipgloss.NewStyle().Foreground(c).Render(verb)
	}
	if folds && f.highlight == key {
		who = highlightIf(true, authorName(r.Author)+" "+verb)
	}
	header := lipgloss.NewStyle().Foreground(c).Render(glyph) + " " + who + m.muted(statusSeparator+m.timestamp(r.SubmittedAt))
	if r.State == domain.ReviewStateDismissed {
		header += m.muted(statusSeparator + "dismissed")
	}
	if !open {
		var hidden []string
		if n := len(body.lines); n > 0 {
			hidden = append(hidden, plural(n, "line"))
		}
		if n := len(threads); n > 0 {
			hidden = append(hidden, plural(n, "thread"))
		}
		header += m.muted(statusSeparator + "▸ " + strings.Join(hidden, statusSeparator))
	}
	p := text(indentedWrap(0, header, width))
	if folds {
		p.folds = []foldable{{key: key, open: open}}
	}
	if !open {
		return p
	}
	p = p.then(body)
	for i, t := range threads {
		if i > 0 || len(body.lines) > 0 {
			p = p.then(text(""))
		}
		p = p.then(m.threadLines(*t, 1, f, width))
	}
	return p
}

// reviewVerb is what a reviewer did, as a review's heading says it.
func reviewVerb(state domain.ReviewState) string {
	switch state {
	case domain.ReviewStateApproved:
		return "approved"
	case domain.ReviewStateChangesRequested:
		return "requested changes"
	}
	return "reviewed"
}

// threadLines draws a review thread at an indent depth: where it is, whether
// it is resolved, and its comments beneath, a blank line apart, unless it is
// folded. A resolved thread starts folded unless resolved threads are
// expanded.
func (m Model) threadLines(t domain.Thread, depth int, f folding, width int) page {
	key := threadFold(t.ID)
	open := f.isOpen(key, !t.Resolved || f.expandResolved)
	marker, state := "▾", lipgloss.NewStyle().Foreground(m.theme.pending).Render("Unresolved")
	if t.Resolved {
		state = lipgloss.NewStyle().Foreground(m.theme.success).Render("Resolved")
	}
	if !open {
		marker = "▸"
	}
	where := m.bold(oneLine(t.Location()))
	if f.highlight == key {
		where = highlightIf(true, oneLine(t.Location()))
	}
	header := m.muted(marker) + " " + where + m.muted(statusSeparator) + state
	if open {
		header += m.muted(statusSeparator + m.timestamp(t.StartedAt()))
	} else {
		header += m.muted(statusSeparator + plural(len(t.Comments), "comment"))
	}
	// The e hint names what e will do to this thread, so it goes once the
	// viewer has folded or unfolded the thread themselves.
	if t.Resolved && !f.chosen(key) {
		verb := "expand"
		if open {
			verb = "collapse"
		}
		header += m.muted(statusSeparator + m.keys.Modal.ExpandResolved.Help().Key + " to " + verb)
	}
	p := text(indentedWrap(depth, header, width))
	p.folds = []foldable{{key: key, open: open}}
	if !open {
		return p
	}
	for _, c := range t.Comments {
		p = p.then(text(""), m.commentLines(c, "", depth+1, width))
	}
	return p
}

// markdownLines renders markdown at an indent depth, recording where each
// link lands. Only http and https links are drawn as links; anything else is
// plain text, since it will never be opened.
func (m Model) markdownLines(src string, depth, width int) page {
	indent := strings.Repeat(" ", depth*indentStep)
	var p page
	for row, l := range markdown.Render(src, width-len(indent)) {
		var b strings.Builder
		b.WriteString(indent)
		col := len(indent)
		for _, sp := range l {
			w := lipgloss.Width(sp.Text)
			if !links.Openable(sp.URL) {
				sp.URL = ""
			}
			if sp.URL != "" {
				p.links = addRegion(p.links, links.Region{Line: row, Start: col, End: col + w, URL: sp.URL})
			}
			b.WriteString(m.spanStyle(sp).Render(sp.Text))
			col += w
		}
		p.lines = append(p.lines, b.String())
	}
	return p
}

// addRegion adds r, joining it to the last region when it continues the same
// link on the same line, as a link with bold text in it does.
func addRegion(regions links.Map, r links.Region) links.Map {
	if n := len(regions); n > 0 {
		last := regions[n-1]
		if last.Line == r.Line && last.End == r.Start && last.URL == r.URL {
			last.End = r.End
			return append(regions[:n-1:n-1], last)
		}
	}
	return append(regions, r)
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
		s = m.asLink(s)
	}
	return s
}

// asLink draws s the way anything clickable is drawn.
func (m Model) asLink(s lipgloss.Style) lipgloss.Style {
	return s.Underline(true).Foreground(m.theme.accent)
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

// copilotReviewer is the login GitHub gives Copilot's code review, which its
// own pages show as "Copilot".
const copilotReviewer = "copilot-pull-request-reviewer"

// authorName is a login, or "ghost" for a deleted account, as GitHub shows it.
func authorName(login string) string {
	switch login {
	case "":
		return "ghost"
	case copilotReviewer:
		return "Copilot"
	}
	return oneLine(login)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
