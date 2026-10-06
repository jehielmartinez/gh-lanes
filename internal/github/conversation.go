package github

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// conversationFragment is what the detail query adds to a pull request. The
// connections are aliased because PullRequestFields already selects comments
// and reviews for their counts. Threads carry no diff hunk and no commit,
// since lanes never shows code.
const conversationFragment = `
fragment ConversationFields on PullRequest {
  body
  conversationComments: comments(first: 100) { ...CommentFields }
  conversationReviews: reviews(first: 100) { ...ReviewFields }
  reviewThreads(first: 50) { ...ReviewThreadFields }
}` + commentFragment + reviewFragment + threadFragment + threadCommentFragment

const commentFragment = `
fragment CommentFields on IssueCommentConnection {
  pageInfo { hasNextPage endCursor }
  nodes { author { login } body createdAt }
}`

const reviewFragment = `
fragment ReviewFields on PullRequestReviewConnection {
  pageInfo { hasNextPage endCursor }
  nodes { author { login } state body submittedAt }
}`

// threadFragment needs threadCommentFragment alongside it.
const threadFragment = `
fragment ReviewThreadFields on PullRequestReviewThreadConnection {
  pageInfo { hasNextPage endCursor }
  nodes {
    id
    path
    line
    originalLine
    isResolved
    comments(first: 50) { ...ThreadCommentFields }
  }
}`

const threadCommentFragment = `
fragment ThreadCommentFields on PullRequestReviewCommentConnection {
  pageInfo { hasNextPage endCursor }
  nodes { author { login } body createdAt }
}`

const pullRequestCommentsQuery = `query PullRequestComments($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequest { conversationComments: comments(first: 100, after: $after) { ...CommentFields } }
  }
  ...RateLimitFields
}` + commentFragment + rateLimitFragment

const pullRequestReviewsQuery = `query PullRequestReviews($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequest { conversationReviews: reviews(first: 100, after: $after) { ...ReviewFields } }
  }
  ...RateLimitFields
}` + reviewFragment + rateLimitFragment

const pullRequestReviewThreadsQuery = `query PullRequestReviewThreads($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequest { reviewThreads(first: 50, after: $after) { ...ReviewThreadFields } }
  }
  ...RateLimitFields
}` + threadFragment + threadCommentFragment + rateLimitFragment

const reviewThreadCommentsQuery = `query ReviewThreadComments($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequestReviewThread { comments(first: 100, after: $after) { ...ThreadCommentFields } }
  }
  ...RateLimitFields
}` + threadCommentFragment + rateLimitFragment

// maxConversationPages bounds how far any one connection is paged, so a
// server that keeps reporting more pages can't hold the modal forever.
const maxConversationPages = 100

type connection[N any] struct {
	PageInfo pageInfo
	Nodes    []N
}

type commentNode struct {
	Author    *login
	Body      string
	CreatedAt time.Time
}

type reviewNode struct {
	Author      *login
	State       string
	Body        string
	SubmittedAt time.Time
}

type threadNode struct {
	ID           string
	Path         string
	Line         int
	OriginalLine int
	IsResolved   bool
	Comments     connection[commentNode]
}

type conversationNode struct {
	Body                 string
	ConversationComments connection[commentNode]
	ConversationReviews  connection[reviewNode]
	ReviewThreads        connection[threadNode]
}

type pullRequestDetailNode struct {
	pullRequestNode
	conversationNode
}

type nodeResponse[T any] struct {
	Node *T
	rateLimitResponse
}

// conversation pages through whatever of the pull request's conversation
// didn't fit on the detail query's first pages. The rate limit is zero when
// nothing more was fetched.
func (c *Client) conversation(ctx context.Context, id string, n conversationNode) (domain.Conversation, domain.RateLimit, error) {
	var limit domain.RateLimit
	keep := func(l domain.RateLimit) {
		if l != (domain.RateLimit{}) {
			limit = l
		}
	}

	comments, l, err := pageThrough(ctx, c, pullRequestCommentsQuery, id, n.ConversationComments,
		func(n conversationNode) connection[commentNode] { return n.ConversationComments })
	if err != nil {
		return domain.Conversation{}, domain.RateLimit{}, fmt.Errorf("comments: %w", err)
	}
	keep(l)
	reviews, l, err := pageThrough(ctx, c, pullRequestReviewsQuery, id, n.ConversationReviews,
		func(n conversationNode) connection[reviewNode] { return n.ConversationReviews })
	if err != nil {
		return domain.Conversation{}, domain.RateLimit{}, fmt.Errorf("reviews: %w", err)
	}
	keep(l)
	threadNodes, l, err := pageThrough(ctx, c, pullRequestReviewThreadsQuery, id, n.ReviewThreads,
		func(n conversationNode) connection[threadNode] { return n.ReviewThreads })
	if err != nil {
		return domain.Conversation{}, domain.RateLimit{}, fmt.Errorf("review threads: %w", err)
	}
	keep(l)

	conv := domain.Conversation{Body: n.Body}
	for _, cn := range comments {
		conv.Comments = append(conv.Comments, cn.toDomain())
	}
	for _, r := range reviews {
		// A pending review is the viewer's own unsubmitted draft.
		if r.State == "PENDING" {
			continue
		}
		conv.Reviews = append(conv.Reviews, domain.Review{
			Author:      r.Author.name(),
			State:       r.State,
			Body:        r.Body,
			SubmittedAt: r.SubmittedAt,
		})
	}
	for _, t := range threadNodes {
		threadComments, l, err := pageThrough(ctx, c, reviewThreadCommentsQuery, t.ID, t.Comments,
			func(n threadNode) connection[commentNode] { return n.Comments })
		if err != nil {
			return domain.Conversation{}, domain.RateLimit{}, fmt.Errorf("review thread comments: %w", err)
		}
		keep(l)
		thread := domain.Thread{Path: t.Path, Line: t.Line, Resolved: t.IsResolved}
		// line is null once the code the thread was on has changed.
		if thread.Line == 0 {
			thread.Line = t.OriginalLine
		}
		for _, cn := range threadComments {
			thread.Comments = append(thread.Comments, cn.toDomain())
		}
		conv.Threads = append(conv.Threads, thread)
	}
	return conv, limit, nil
}

func (n commentNode) toDomain() domain.Comment {
	return domain.Comment{Author: n.Author.name(), Body: n.Body, CreatedAt: n.CreatedAt}
}

// pageThrough returns every node of a connection, fetching the pages after
// first with query, which selects the same connection on node id. The rate
// limit is zero when there was nothing more to fetch.
func pageThrough[T, N any](ctx context.Context, c *Client, query, id string, first connection[N], conn func(T) connection[N]) ([]N, domain.RateLimit, error) {
	nodes := slices.Clone(first.Nodes)
	var limit domain.RateLimit
	page := first.PageInfo
	for range maxConversationPages {
		if !page.HasNextPage || page.EndCursor == "" {
			break
		}
		var resp nodeResponse[T]
		if err := c.do(ctx, query, map[string]any{"id": id, "after": page.EndCursor}, &resp); err != nil {
			return nil, domain.RateLimit{}, err
		}
		limit = resp.toDomain()
		if resp.Node == nil {
			break
		}
		next := conn(*resp.Node)
		nodes = append(nodes, next.Nodes...)
		page = next.PageInfo
	}
	return nodes, limit, nil
}
