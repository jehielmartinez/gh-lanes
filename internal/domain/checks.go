package domain

import (
	"cmp"
	"slices"
	"time"
)

// Check is one check run or commit status on a pull request's head commit.
type Check struct {
	Kind CheckKind
	Name string
	// Workflow is the Actions workflow that ran the check, empty for commit
	// statuses and third-party check runs.
	Workflow string
	Outcome  CheckOutcome
	// StartedAt is zero for a check run still queued. For a commit status it
	// is when the status was set.
	StartedAt   time.Time
	CompletedAt time.Time
	DetailsURL  string
}

// CheckKind is which of GitHub's two check APIs reported a check.
type CheckKind int

// The check kinds.
const (
	CheckRun CheckKind = iota
	CommitStatus
)

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
	Passed, Failed, Pending, Skipped int
}

// CountChecks tallies checks by outcome.
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
		case CheckSkipped:
			c.Skipped++
		}
	}
	return c
}

// CheckGroup is the checks of one workflow, or every commit status, or every
// check run that came from no workflow.
type CheckGroup struct {
	Kind CheckKind
	// Workflow is empty for commit statuses and for check runs outside
	// Actions.
	Workflow string
	Checks   []Check
}

// GroupChecks groups checks by workflow, with commit statuses in a group of
// their own. Failures sort first, then pending, passed and skipped checks,
// both within a group and between groups, which compare by their most urgent
// check.
func GroupChecks(checks []Check) []CheckGroup {
	type key struct {
		kind     CheckKind
		workflow string
	}
	index := map[key]int{}
	var groups []CheckGroup
	for _, ch := range checks {
		k := key{ch.Kind, ch.Workflow}
		if ch.Kind == CommitStatus {
			k.workflow = ""
		}
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, CheckGroup{Kind: k.kind, Workflow: k.workflow})
		}
		groups[i].Checks = append(groups[i].Checks, ch)
	}
	for i := range groups {
		slices.SortStableFunc(groups[i].Checks, func(a, b Check) int {
			return cmp.Or(cmp.Compare(urgency(a.Outcome), urgency(b.Outcome)), cmp.Compare(a.Name, b.Name))
		})
	}
	slices.SortStableFunc(groups, func(a, b CheckGroup) int {
		return cmp.Or(
			cmp.Compare(urgency(a.Checks[0].Outcome), urgency(b.Checks[0].Outcome)),
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(unnamedLast(a.Workflow), unnamedLast(b.Workflow)),
			cmp.Compare(a.Workflow, b.Workflow),
		)
	})
	return groups
}

func urgency(o CheckOutcome) int {
	switch o {
	case CheckFailed:
		return 0
	case CheckPending:
		return 1
	case CheckPassed:
		return 2
	}
	return 3
}

// unnamedLast sorts check runs outside any workflow after the named workflows.
func unnamedLast(workflow string) int {
	if workflow == "" {
		return 1
	}
	return 0
}

// CheckState is the one-word verdict of all of a pull request's checks
// together.
type CheckState string

// The check states. CheckStateNone means there are no checks, or none that
// reached a verdict.
const (
	CheckStateNone    CheckState = ""
	CheckStatePending CheckState = "PENDING"
	CheckStatePassed  CheckState = "PASSED"
	CheckStateFailed  CheckState = "FAILED"
)

// OverallCheckState folds checks into one state: any failure fails them all,
// then anything still running keeps them pending.
func OverallCheckState(checks []Check) CheckState {
	c := CountChecks(checks)
	switch {
	case c.Failed > 0:
		return CheckStateFailed
	case c.Pending > 0:
		return CheckStatePending
	case c.Passed > 0:
		return CheckStatePassed
	}
	return CheckStateNone
}
