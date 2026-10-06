// Package activity decides which pull requests have changed since they were
// last seen, and which are new.
package activity

import (
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

// Baseline records every pull request in prs as seen at now, so nothing on
// the board at the first run counts as new.
func Baseline(prs []domain.PullRequest, now time.Time) map[string]domain.Snapshot {
	snapshots := make(map[string]domain.Snapshot, len(prs))
	for _, pr := range prs {
		snapshots[pr.ID] = Take(pr, now, domain.Snapshot{}, false)
	}
	return snapshots
}

// Of decides the marker for pr given the snapshots taken so far.
func Of(pr domain.PullRequest, snapshots map[string]domain.Snapshot) Marker {
	snap, ok := snapshots[pr.ID]
	switch {
	case !ok:
		return New
	case differs(snap, pr):
		return Changed
	}
	return None
}

func differs(snap domain.Snapshot, pr domain.PullRequest) bool {
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
func Tally(markers map[string]Marker) (changed, added int) {
	for _, m := range markers {
		switch m {
		case Changed:
			changed++
		case New:
			added++
		}
	}
	return changed, added
}
