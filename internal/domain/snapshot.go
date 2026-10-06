package domain

import "time"

// Snapshot is what a pull request looked like when it was last seen, kept to
// tell whether anything important has changed since.
type Snapshot struct {
	SeenAt         time.Time
	Checks         CheckState
	Mergeable      Mergeable
	ReviewDecision ReviewDecision
	Comments       int
	Reviews        int
	Draft          bool
	State          State
}
