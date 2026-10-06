package ui

import (
	"context"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// archivedMsg is the result of fetching the archived pull requests by node
// ID.
type archivedMsg struct {
	seq       int
	prs       []domain.PullRequest
	rateLimit domain.RateLimit
	err       error
}

// fetchArchived loads the archived pull requests. They are fetched only for
// the archived tab, so a board refresh spends nothing on them.
func (m Model) fetchArchived() (Model, tea.Cmd) {
	if !m.storeReady || m.archiveFetching {
		return m, nil
	}
	if len(m.archived) == 0 {
		m.archive, m.archiveLoaded, m.archiveErr = cardList{}, true, nil
		return m, nil
	}
	ids := make([]string, len(m.archived))
	for i, a := range m.archived {
		ids[i] = a.ID
	}
	m.archiveSeq++
	m.archiveFetching = true
	gh, seq := m.opts.GitHub, m.archiveSeq
	fetch := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, limit, err := gh.PullRequestsByID(ctx, ids)
		return archivedMsg{seq: seq, prs: prs, rateLimit: limit, err: err}
	}
	return m, tea.Batch(fetch, m.spinner.Tick)
}

// archivedFetched takes in the archived pull requests, leaving out any
// unarchived while the fetch was in flight. A failed fetch keeps the list it
// had.
func (m Model) archivedFetched(msg archivedMsg) Model {
	if msg.seq != m.archiveSeq {
		return m
	}
	m.archiveFetching = false
	if msg.rateLimit != (domain.RateLimit{}) {
		m.rateLimit = msg.rateLimit
	}
	m.archiveErr = msg.err
	if msg.err != nil {
		return m
	}
	prs := slices.DeleteFunc(slices.Clone(msg.prs), func(pr domain.PullRequest) bool {
		return !board.IsArchived(m.archived, pr.ID)
	})
	m.archive, m.archiveLoaded = m.archive.withPRs(prs), true
	return m
}

// archivedView is the archived tab: the archived pull requests, or why there
// are none to show.
func (m Model) archivedView() string {
	muted := lipgloss.NewStyle().Foreground(m.theme.muted)
	errLine := ""
	if m.archiveErr != nil {
		errLine = lipgloss.NewStyle().Foreground(m.theme.errText).Render("Couldn't load archived pull requests: " + oneLine(m.archiveErr.Error()))
	}
	switch {
	case m.storeReady && len(m.archived) == 0:
		return muted.Render("Nothing archived. Press " + m.keys.Archive.Help().Key + " on a board card to archive it.")
	case !m.archiveLoaded && errLine != "":
		return errLine
	case !m.archiveLoaded:
		return m.spinner.View() + " " + muted.Render("Loading archived pull requests…")
	case errLine != "":
		return lipgloss.JoinVertical(lipgloss.Left, errLine, m.listView(m.archive, ""))
	}
	return m.listView(m.archive, "")
}

// unarchiveSelected puts the selected archived pull request back on the
// board, in the lane it was archived from, and saves that.
func (m Model) unarchiveSelected() (Model, tea.Cmd) {
	pr, ok := m.archive.selected()
	if !ok || !m.storeReady {
		return m, nil
	}
	var tagID string
	m.assignments, m.archived, tagID = board.Unarchive(m.assignments, m.archived, m.tags, pr.ID)
	m.archive = m.archive.withPRs(slices.DeleteFunc(slices.Clone(m.archive.prs), func(p domain.PullRequest) bool { return p.ID == pr.ID }))
	// A finished pull request in Untagged is never kept, so it doesn't come
	// back; anything else waits on the board for the next refresh to update.
	onBoard := tagID != "" || !pr.Finished()
	if onBoard && m.boardIndex(pr.ID) < 0 {
		m.prs = append(slices.Clone(m.prs), pr)
	}
	text := ref(pr) + ": restored to " + m.laneName(tagID)
	if !onBoard {
		text = ref(pr) + ": unarchived, but it is " + finishedWord(pr) + " and untagged, so it stays off the board"
	}
	m.toast = toast{text: text, until: m.now.Add(toastDuration)}
	return m.rebuild().saveState()
}

// laneName is the name of the lane a tag ID puts a pull request in.
func (m Model) laneName(tagID string) string {
	for _, t := range m.tags {
		if t.ID == tagID {
			return t.Name
		}
	}
	return board.UntaggedName
}

func finishedWord(pr domain.PullRequest) string {
	if pr.State == domain.StateMerged {
		return "merged"
	}
	return "closed"
}
