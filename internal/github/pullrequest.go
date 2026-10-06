package github

import (
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// pullRequestFragment is the one selection of pull request fields, shared by
// every query that returns pull requests.
//
// latestReviews holds one review per reviewer, so its first page is all of it
// in practice and it is not paged.
const pullRequestFragment = `
fragment PullRequestFields on PullRequest {
  id
  number
  title
  url
  isDraft
  state
  createdAt
  updatedAt
  mergedAt
  closedAt
  author { login }
  baseRefName
  headRefName
  mergeable
  mergeStateStatus
  reviewDecision
  viewerCanUpdate
  autoMergeRequest { mergeMethod enabledAt enabledBy { login } }
  repository {
    nameWithOwner
    mergeCommitAllowed
    squashMergeAllowed
    rebaseMergeAllowed
    autoMergeAllowed
    deleteBranchOnMerge
  }
  comments { totalCount }
  reviews { totalCount }
  latestReviews(first: 100) { nodes { author { login } state submittedAt } }
  commits(last: 1) {
    nodes {
      commit {
        id
        statusCheckRollup {
          contexts(first: 100) { ...CheckContextFields }
        }
      }
    }
  }
}`

const checkContextFragment = `
fragment CheckContextFields on StatusCheckRollupContextConnection {
  pageInfo { hasNextPage endCursor }
  nodes {
    __typename
    ... on CheckRun {
      name
      status
      conclusion
      startedAt
      completedAt
      detailsUrl
      checkSuite { workflowRun { workflow { name } } }
    }
    ... on StatusContext {
      context
      state
      targetUrl
      createdAt
    }
  }
}`

const rateLimitFragment = `
fragment RateLimitFields on Query {
  rateLimit { limit remaining resetAt }
}`

type pageInfo struct {
	HasNextPage bool
	EndCursor   string
}

type login struct {
	Login string
}

type totalCount struct {
	TotalCount int
}

type pullRequestNode struct {
	ID               string
	Number           int
	Title            string
	URL              string
	IsDraft          bool
	State            string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	MergedAt         time.Time
	ClosedAt         time.Time
	Author           *login
	BaseRefName      string
	HeadRefName      string
	Mergeable        string
	MergeStateStatus string
	ReviewDecision   string
	ViewerCanUpdate  bool
	AutoMergeRequest *struct {
		MergeMethod string
		EnabledAt   time.Time
		EnabledBy   *login
	}
	Repository struct {
		NameWithOwner       string
		MergeCommitAllowed  bool
		SquashMergeAllowed  bool
		RebaseMergeAllowed  bool
		AutoMergeAllowed    bool
		DeleteBranchOnMerge bool
	}
	Comments      totalCount
	Reviews       totalCount
	LatestReviews struct {
		Nodes []struct {
			Author      *login
			State       string
			SubmittedAt time.Time
		}
	}
	Commits struct {
		Nodes []struct {
			Commit struct {
				ID                string
				StatusCheckRollup *statusCheckRollup
			}
		}
	}
}

type statusCheckRollup struct {
	Contexts checkContexts
}

type checkContexts struct {
	PageInfo pageInfo
	Nodes    []checkContextNode
}

type checkContextNode struct {
	Typename string `json:"__typename"`

	Name        string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
	DetailsURL  string
	CheckSuite  *struct {
		WorkflowRun *struct {
			Workflow struct {
				Name string
			}
		}
	}

	Context   string
	State     string
	TargetURL string
	CreatedAt time.Time
}

type rateLimitResponse struct {
	RateLimit *struct {
		Limit     int
		Remaining int
		ResetAt   time.Time
	}
}

func (r rateLimitResponse) toDomain() domain.RateLimit {
	if r.RateLimit == nil {
		return domain.RateLimit{}
	}
	return domain.RateLimit{Limit: r.RateLimit.Limit, Remaining: r.RateLimit.Remaining, ResetAt: r.RateLimit.ResetAt}
}

func (n pullRequestNode) rollup() *statusCheckRollup {
	if len(n.Commits.Nodes) == 0 {
		return nil
	}
	return n.Commits.Nodes[0].Commit.StatusCheckRollup
}

func (n pullRequestNode) headCommitID() string {
	if len(n.Commits.Nodes) == 0 {
		return ""
	}
	return n.Commits.Nodes[0].Commit.ID
}

func (n pullRequestNode) toDomain() domain.PullRequest {
	pr := domain.PullRequest{
		ID:      n.ID,
		Number:  n.Number,
		Title:   n.Title,
		URL:     n.URL,
		Author:  n.Author.name(),
		State:   domain.State(n.State),
		IsDraft: n.IsDraft,
		Repository: domain.Repository{
			NameWithOwner:       n.Repository.NameWithOwner,
			MergeMethods:        n.mergeMethods(),
			AutoMergeAllowed:    n.Repository.AutoMergeAllowed,
			DeleteBranchOnMerge: n.Repository.DeleteBranchOnMerge,
		},
		CreatedAt:        n.CreatedAt,
		UpdatedAt:        n.UpdatedAt,
		MergedAt:         n.MergedAt,
		ClosedAt:         n.ClosedAt,
		BaseRef:          n.BaseRefName,
		HeadRef:          n.HeadRefName,
		Mergeable:        domain.Mergeable(n.Mergeable),
		MergeStateStatus: domain.MergeStateStatus(n.MergeStateStatus),
		ReviewDecision:   domain.ReviewDecision(n.ReviewDecision),
		ViewerCanUpdate:  n.ViewerCanUpdate,
		CommentCount:     n.Comments.TotalCount,
		ReviewCount:      n.Reviews.TotalCount,
	}
	if am := n.AutoMergeRequest; am != nil {
		pr.AutoMerge = &domain.AutoMerge{
			Method:    domain.MergeMethod(am.MergeMethod),
			EnabledBy: am.EnabledBy.name(),
			EnabledAt: am.EnabledAt,
		}
	}
	for _, r := range n.LatestReviews.Nodes {
		pr.LatestReviews = append(pr.LatestReviews, domain.Review{
			Author:      r.Author.name(),
			State:       r.State,
			SubmittedAt: r.SubmittedAt,
		})
	}
	if rollup := n.rollup(); rollup != nil {
		pr.Checks = rollup.Contexts.toDomain()
	}
	return pr
}

// name returns the login, or "" for a deleted account, which GitHub reports
// as a null author.
func (l *login) name() string {
	if l == nil {
		return ""
	}
	return l.Login
}

func (n pullRequestNode) mergeMethods() []domain.MergeMethod {
	var methods []domain.MergeMethod
	if n.Repository.MergeCommitAllowed {
		methods = append(methods, domain.MergeMethodMerge)
	}
	if n.Repository.SquashMergeAllowed {
		methods = append(methods, domain.MergeMethodSquash)
	}
	if n.Repository.RebaseMergeAllowed {
		methods = append(methods, domain.MergeMethodRebase)
	}
	return methods
}

func (c checkContexts) toDomain() []domain.Check {
	checks := make([]domain.Check, 0, len(c.Nodes))
	for _, n := range c.Nodes {
		switch n.Typename {
		case "CheckRun":
			check := domain.Check{
				Name:        n.Name,
				Outcome:     checkRunOutcome(n.Status, n.Conclusion),
				StartedAt:   n.StartedAt,
				CompletedAt: n.CompletedAt,
				DetailsURL:  n.DetailsURL,
			}
			if n.CheckSuite != nil && n.CheckSuite.WorkflowRun != nil {
				check.Workflow = n.CheckSuite.WorkflowRun.Workflow.Name
			}
			checks = append(checks, check)
		case "StatusContext":
			checks = append(checks, domain.Check{
				Name:       n.Context,
				Outcome:    statusContextOutcome(n.State),
				StartedAt:  n.CreatedAt,
				DetailsURL: n.TargetURL,
			})
		}
	}
	return checks
}

func checkRunOutcome(status, conclusion string) domain.CheckOutcome {
	if status != "COMPLETED" {
		return domain.CheckPending
	}
	switch conclusion {
	case "SUCCESS", "NEUTRAL":
		return domain.CheckPassed
	case "SKIPPED":
		return domain.CheckSkipped
	}
	return domain.CheckFailed
}

func statusContextOutcome(state string) domain.CheckOutcome {
	switch state {
	case "SUCCESS":
		return domain.CheckPassed
	case "PENDING", "EXPECTED":
		return domain.CheckPending
	}
	return domain.CheckFailed
}
