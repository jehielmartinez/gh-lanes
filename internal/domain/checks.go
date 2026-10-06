package domain

import "time"

// Check is one check run or commit status on a pull request's head commit.
type Check struct {
	Name string
	// Workflow is the Actions workflow that ran the check, empty for commit
	// statuses and third-party check runs.
	Workflow    string
	Outcome     CheckOutcome
	StartedAt   time.Time
	CompletedAt time.Time
	DetailsURL  string
}

// CheckOutcome is where a check stands, folded from GitHub's status,
// conclusion and commit-status state.
type CheckOutcome int

// The check outcomes.
const (
	CheckPending CheckOutcome = iota
	CheckPassed
	CheckFailed
	// CheckSkipped covers checks that ran to no verdict, so they count as
	// neither passed nor failed.
	CheckSkipped
)

// CheckCounts tallies a pull request's checks by outcome.
type CheckCounts struct {
	Passed, Failed, Pending int
}

// CountChecks tallies checks by outcome. Skipped checks are left out.
func CountChecks(checks []Check) CheckCounts {
	var c CheckCounts
	for _, ch := range checks {
		switch ch.Outcome {
		case CheckPassed:
			c.Passed++
		case CheckFailed:
			c.Failed++
		case CheckPending:
			c.Pending++
		}
	}
	return c
}
