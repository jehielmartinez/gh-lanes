package ui

import (
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/jehielmartinez/gh-lanes/internal/activity"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// observe records what has been seen since the last message and decides the
// cards' markers, saving the state file when a snapshot was taken.
func (m Model) observe() (Model, tea.Cmd) {
	changed := false
	if m.storeReady && m.loaded && m.snapshots == nil {
		m.snapshots = activity.Baseline(m.prs, m.updatedAt)
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
	pr := m.detail.pr
	if i := m.boardIndex(pr.ID); i >= 0 {
		pr = m.prs[i]
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

// withBoardCopy replaces the board's copy of a pull request with a fresher
// one, keeping the selection on the card it was on.
func (m Model) withBoardCopy(pr domain.PullRequest) Model {
	i := m.boardIndex(pr.ID)
	if i < 0 {
		return m
	}
	selected, hadSelection := m.selected()
	prs := make([]domain.PullRequest, len(m.prs))
	copy(prs, m.prs)
	prs[i] = pr
	m.prs = prs
	m = m.rebuild()
	if hadSelection {
		m = m.reselect(selected.ID)
	}
	return m
}
