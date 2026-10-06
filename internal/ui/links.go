package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/links"
)

// page is laid-out modal content: its lines, and where each link is drawn
// on them.
type page struct {
	lines []string
	links links.Map
}

// text is a page of plain text; each string may hold several lines.
func text(blocks ...string) page {
	var p page
	for _, b := range blocks {
		p.lines = append(p.lines, strings.Split(b, "\n")...)
	}
	return p
}

// then returns the page with more placed below it.
func (p page) then(more ...page) page {
	for _, q := range more {
		p.links = append(p.links[:len(p.links):len(p.links)], q.links.Shift(len(p.lines))...)
		p.lines = append(p.lines[:len(p.lines):len(p.lines)], q.lines...)
	}
	return p
}

// openMsg asks for a URL to be opened in the browser.
type openMsg struct{ url string }

// openedMsg is the opener's verdict on a URL.
type openedMsg struct{ err error }

// linksMsg is the fresh copy of a pull request whose links were asked for
// from the board.
type linksMsg struct {
	seq       int
	pr        domain.PullRequest
	rateLimit domain.RateLimit
	err       error
}

// linkFailure is why the last link couldn't be loaded or opened.
type linkFailure struct {
	// action is what failed, as the status bar starts the sentence.
	action string
	err    error
}

// open hands a URL to the opener, off the UI thread.
func (m Model) open(url string) (Model, tea.Cmd) {
	// The links drawn and listed are already filtered; this guards the
	// opener against any other way a URL might reach it.
	if !links.Openable(url) {
		m.linkErr = &linkFailure{action: "Couldn't open link", err: links.ErrNotOpenable}
		return m, nil
	}
	m.linkErr = nil
	opener := m.opts.Open
	return m, func() tea.Msg { return openedMsg{err: opener(url)} }
}

// openLinks opens the link picker on the modal's pull request, or, from the
// board, fetches the selected one first, since the board's copy carries no
// conversation.
func (m Model) openLinks() (Model, tea.Cmd) {
	if m.detail != nil {
		return m.linkPicker(m.detail.pr), nil
	}
	pr, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.detailSeq++
	m.linksSeq, m.linkErr = m.detailSeq, nil
	gh, seq, id := m.opts.GitHub, m.detailSeq, pr.ID
	fetch := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		pr, limit, err := gh.PullRequest(ctx, id)
		return linksMsg{seq: seq, pr: pr, rateLimit: limit, err: err}
	}
	return m, tea.Batch(fetch, m.spinner.Tick)
}

// linksFetched opens the picker on the fetched pull request, unless the
// request was overtaken or something else has opened meanwhile.
func (m Model) linksFetched(msg linksMsg) Model {
	if msg.seq != m.linksSeq {
		return m
	}
	m.linksSeq = 0
	if msg.rateLimit != (domain.RateLimit{}) {
		m.rateLimit = msg.rateLimit
	}
	if msg.err != nil {
		m.linkErr = &linkFailure{action: "Couldn't load links", err: msg.err}
		return m
	}
	if m.detail != nil || m.picker != nil || m.tagManager != nil {
		return m
	}
	return m.linkPicker(msg.pr)
}

// linkPicker lists every link in the pull request with where it came from.
func (m Model) linkPicker(pr domain.PullRequest) Model {
	found := links.Collect(pr)
	items := make([]pickerItem, len(found))
	for i, l := range found {
		note := linkSource(l)
		label := l.URL
		if m.width > 0 {
			label = truncate(label, max(m.width-lipgloss.Width(note)-pickerFrame, 20))
		}
		items[i] = pickerItem{label: label, note: note}
	}
	m.picker = newPicker("Links", items, 0, func(i int) tea.Msg { return openMsg{url: found[i].URL} })
	m.picker.rows = m.pickerRows()
	m.picker.empty = "No links in this pull request."
	return m
}

// pickerFrame is the width a picker adds around an item's label and note:
// border, padding, cursor marker and the gap before the note.
const pickerFrame = 2 + 4 + 2 + 1

// pickerRows is how many items a picker shows at once on this screen, or 0
// while the screen's size is unknown.
func (m Model) pickerRows() int {
	if m.height == 0 {
		return 0
	}
	// The picker's border, title, help line and the blank lines around them.
	return max(m.height-6, 1)
}

func linkSource(l links.Link) string {
	where := map[links.Source]string{
		links.FromCheck:       "check",
		links.FromDescription: "description",
		links.FromComment:     "comment",
		links.FromReview:      "review",
		links.FromThread:      "thread",
	}[l.Source]
	if l.Where != "" {
		where += " " + oneLine(l.Where)
	}
	if l.Source == links.FromCheck {
		return where
	}
	return where + statusSeparator + authorName(l.Author)
}

// clickDetail opens the link under a left click in the modal.
func (m Model) clickDetail(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	width, height := m.modalSize()
	x, y := m.overlayOrigin(width, height)
	// The modal's content starts inside its border and left padding.
	col, row := msg.X-x-2, msg.Y-y-1
	vp := m.detail.viewport
	if col < 0 || col >= vp.Width() || row < 0 || row >= vp.Height() {
		return m, nil
	}
	if url, ok := m.detail.links.At(row+vp.YOffset(), col); ok {
		return m.open(url)
	}
	return m, nil
}

// linkLine is the status bar's report of the last link that couldn't be
// loaded or opened, empty when there is none.
func (m Model) linkLine() string {
	if m.linkErr == nil {
		return ""
	}
	return lipgloss.NewStyle().Foreground(m.theme.errText).Render(m.linkErr.action + ": " + oneLine(m.linkErr.err.Error()))
}

// loadingLinks is whether a fetch for the board's link picker is in flight.
func (m Model) loadingLinks() bool { return m.linksSeq != 0 }

// openPullRequest opens the modal's pull request, or the selected card's, on
// GitHub.
func (m Model) openPullRequest() (Model, tea.Cmd) {
	if m.detail != nil {
		return m.open(m.detail.pr.URL)
	}
	if pr, ok := m.selected(); ok {
		return m.open(pr.URL)
	}
	return m, nil
}
