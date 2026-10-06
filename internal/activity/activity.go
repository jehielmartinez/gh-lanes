// Package activity decides which pull requests have changed since they were
// last seen, and which are new.
package activity

import (
	"maps"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// Marker is what a card shows about activity since its pull request was last
// seen.
type Marker int

// The markers.
const (
	None Marker = iota
	Changed
	// New is a pull request that has never been seen, because it appeared
	// after the first run and hasn't been opened since.
	New
)

// Take records pr as seen at now. prev is the pull request's last snapshot,
// if it had one.
func Take(pr domain.PullRequest, now time.Time, prev domain.Snapshot, hadPrev bool) domain.Snapshot {
	s := domain.Snapshot{
		SeenAt:         now,
		Checks:         domain.OverallCheckState(pr.Checks),
		Mergeable:      pr.Mergeable,
		ReviewDecision: pr.ReviewDecision,
		Comments:       pr.CommentCount,
		Reviews:        pr.ReviewCount,
		Draft:          pr.IsDraft,
		State:          pr.State,
	}
	// GitHub reports UNKNOWN while it recomputes mergeability, so it says
	// nothing about what was seen; the last known value still stands.
	if s.Mergeable == domain.MergeableUnknown && hadPrev {
		s.Mergeable = prev.Mergeable
	}
	return s
}

// Same reports whether two snapshots record the same pull request data,
// whenever each was taken.
func Same(a, b domain.Snapshot) bool {
	a.SeenAt = b.SeenAt
	return a == b
}

// Baseline returns snapshots with every pull request in prs recorded as seen
// at now, except those that already have one, so nothing already on the
// board counts as new.
func Baseline(snapshots map[string]domain.Snapshot, prs []domain.PullRequest, now time.Time) map[string]domain.Snapshot {
	next := maps.Clone(snapshots)
	if next == nil {
		next = map[string]domain.Snapshot{}
	}
	for _, pr := range prs {
		if _, ok := next[pr.ID]; !ok {
			next[pr.ID] = Take(pr, now, domain.Snapshot{}, false)
		}
	}
	return next
}

// Of decides the marker for pr given the snapshots taken so far.
func Of(pr domain.PullRequest, snapshots map[string]domain.Snapshot) Marker {
	snap, ok := snapshots[pr.ID]
	switch {
	case !ok:
		return New
	case changed(snap, pr):
		return Changed
	}
	return None
}

func changed(snap domain.Snapshot, pr domain.PullRequest) bool {
	now := Take(pr, snap.SeenAt, snap, true)
	// A mergeability nobody has seen computed can't have changed.
	if snap.Mergeable == domain.MergeableUnknown {
		now.Mergeable = domain.MergeableUnknown
	}
	return !Same(snap, now)
}

// Markers decides the marker of every pull request in prs that has one.
func Markers(prs []domain.PullRequest, snapshots map[string]domain.Snapshot) map[string]Marker {
	markers := map[string]Marker{}
	for _, pr := range prs {
		if m := Of(pr, snapshots); m != None {
			markers[pr.ID] = m
		}
	}
	return markers
}

// Tally counts the changed and the new pull requests among markers.
func Tally(markers map[string]Marker) (changed, new int) {
	for _, m := range markers {
		switch m {
		case Changed:
			changed++
		case New:
			new++
		}
	}
	return changed, new
}
