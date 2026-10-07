package ui

import (
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/jehielmartinez/gh-lanes/internal/activity"
	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// observe records what has been seen since the last message and decides the
// cards' markers, saving the state file when a snapshot was taken.
func (m Model) observe() (Model, tea.Cmd) {
	changed := false
	if m.storeReady && m.loaded && m.snapshots == nil {
		m.snapshots = activity.Baseline(board.Visible(m.prs, m.filter), m.updatedAt)
		changed = true
	}
	m, seen := m.markSeen()
	var onBoard []domain.PullRequest
	for _, lane := range m.lanes {
		onBoard = append(onBoard, lane.PullRequests...)
	}
	m.markers = activity.Markers(onBoard, m.snapshots)
	if !changed && !seen {
		return m, nil
	}
	return m.saveState()
}

// markSeen records the pull request open in the modal as seen. Opening it
// always takes a snapshot; while it stays open, one is taken whenever what it
// shows changes, so it never comes back to the board marked.
func (m Model) markSeen() (Model, bool) {
	if m.detail == nil || !m.storeReady || m.snapshots == nil {
		return m, false
	}
	// Markers are for the board's own pull requests; a review request is
	// someone else's work and is never snapshotted.
	i := m.boardIndex(m.detail.pr.ID)
	if i < 0 {
		return m, false
	}
	pr := m.prs[i]
	// A hidden pull request's snapshot stays frozen, so its marker shows what
	// changed while it was hidden once it is shown again.
	if board.Excluded(m.filter, pr.Repository.NameWithOwner) {
		return m, false
	}
	prev, had := m.snapshots[pr.ID]
	next := activity.Take(pr, m.opts.Now(), prev, had)
	if m.detail.seen && had && activity.Same(prev, next) {
		return m, false
	}
	m.snapshots = maps.Clone(m.snapshots)
	m.snapshots[pr.ID] = next
	d := *m.detail
	d.seen = true
	m.detail = &d
	return m, true
}

// boardIndex is the index of the pull request in m.prs, or -1.
func (m Model) boardIndex(id string) int {
	for i, pr := range m.prs {
		if pr.ID == id {
			return i
		}
	}
	return -1
}

// withFresherCopy replaces the board's and the review requests' copies of a
// pull request with a fresher one, keeping each selection on the card it was
// on.
func (m Model) withFresherCopy(pr domain.PullRequest) Model {
	m = m.withListCopies(pr)
	if m.boardIndex(pr.ID) < 0 {
		return m
	}
	selected, hadSelection := m.selected()
	m.prs = replaced(m.prs, pr)
	m = m.rebuild()
	if hadSelection {
		m = m.reselect(selected.ID)
	}
	return m
}
