// Package github is the only part of lanes that talks to the network. It
// speaks GraphQL to GitHub through go-gh and maps responses into domain values.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// ErrNotLoggedIn means gh holds no token for the host lanes would talk to.
var ErrNotLoggedIn = errors.New("not logged in")

// ErrLoginRejected means GitHub refused the token gh holds, usually because it
// expired or was revoked.
var ErrLoginRejected = errors.New("login rejected")

// Client runs lanes' GraphQL queries against one GitHub host.
type Client struct {
	gql  *api.GraphQLClient
	host string
}

// New returns a client for the host and token gh is configured with, honouring
// GH_HOST and GH_TOKEN. A nil transport means the default HTTP transport.
func New(transport http.RoundTripper) (*Client, error) {
	if transport == nil {
		transport = http.DefaultTransport
	}
	host, _ := auth.DefaultHost()
	token, _ := auth.TokenForHost(host)
	if token == "" {
		return nil, fmt.Errorf("%w to %s: run `gh auth login` to authenticate", ErrNotLoggedIn, host)
	}
	gql, err := api.NewGraphQLClient(api.ClientOptions{
		Host:      host,
		AuthToken: token,
		Transport: stripTransport{next: transport},
	})
	if err != nil {
		return nil, fmt.Errorf("create GraphQL client for %s: %w", host, err)
	}
	return &Client{gql: gql, host: host}, nil
}

// do runs one GraphQL operation, turning a rejected token into
// ErrLoginRejected so the UI can say how to fix it.
func (c *Client) do(ctx context.Context, query string, vars map[string]any, resp any) error {
	err := c.gql.DoWithContext(ctx, query, vars, resp)
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w by %s: run `gh auth login`", ErrLoginRejected, c.host)
	}
	return err
}

// GitHub search never returns more than 1,000 results, so this many pages
// covers everything a search can produce. Pages are kept small because each
// pull request carries its checks and reviews.
const (
	searchPageSize = 25
	maxSearchPages = 1000 / searchPageSize
)

const searchPullRequestsQuery = `query SearchPullRequests($query: String!, $first: Int!, $after: String) {
  search(query: $query, type: ISSUE, first: $first, after: $after) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest { ...PullRequestFields }
    }
  }
  ...RateLimitFields
}` + pullRequestFragment + checkContextFragment + rateLimitFragment

type searchResponse struct {
	Search struct {
		PageInfo pageInfo
		Nodes    []pullRequestNode
	}
	rateLimitResponse
}

// SearchPullRequests returns every pull request the search string matches,
// paging through the results and through each one's checks, along with the
// rate-limit budget left afterwards.
func (c *Client) SearchPullRequests(ctx context.Context, query string) ([]domain.PullRequest, domain.RateLimit, error) {
	var prs []domain.PullRequest
	var limit domain.RateLimit
	var after *string
	for range maxSearchPages {
		var resp searchResponse
		vars := map[string]any{"query": query, "first": searchPageSize, "after": after}
		if err := c.do(ctx, searchPullRequestsQuery, vars, &resp); err != nil {
			return nil, domain.RateLimit{}, fmt.Errorf("search pull requests: %w", err)
		}
		limit = resp.toDomain()
		found, l, err := c.toDomain(ctx, resp.Search.Nodes)
		if err != nil {
			return nil, domain.RateLimit{}, err
		}
		prs = append(prs, found...)
		limit = latest(limit, l)
		page := resp.Search.PageInfo
		if !page.HasNextPage || page.EndCursor == "" {
			break
		}
		after = &page.EndCursor
	}
	return prs, limit, nil
}

// The nodes query accepts up to 100 IDs; batches are kept to a search page
// for the same reason search pages are small.
const nodesBatchSize = searchPageSize

const pullRequestsByIDQuery = `query PullRequestsByID($ids: [ID!]!) {
  nodes(ids: $ids) {
    ... on PullRequest { ...PullRequestFields }
  }
  ...RateLimitFields
}` + pullRequestFragment + checkContextFragment + rateLimitFragment

const pullRequestDetailQuery = `query PullRequestDetail($id: ID!) {
  node(id: $id) {
    ... on PullRequest { ...PullRequestFields }
  }
  ...RateLimitFields
}` + pullRequestFragment + checkContextFragment + rateLimitFragment

type nodesResponse struct {
	Nodes []pullRequestNode
	rateLimitResponse
}

// PullRequestsByID fetches pull requests by node ID, whatever their state,
// along with the rate-limit budget left afterwards. An ID that no longer
// resolves, such as a deleted pull request or one the login can no longer
// see, is left out rather than failing the rest.
func (c *Client) PullRequestsByID(ctx context.Context, ids []string) ([]domain.PullRequest, domain.RateLimit, error) {
	var prs []domain.PullRequest
	var limit domain.RateLimit
	for batch := range slices.Chunk(ids, nodesBatchSize) {
		var resp nodesResponse
		err := c.do(ctx, pullRequestsByIDQuery, map[string]any{"ids": batch}, &resp)
		var gqlErr *api.GraphQLError
		if err != nil && !(errors.As(err, &gqlErr) && gqlErr.Match("NOT_FOUND", "nodes.")) {
			return nil, domain.RateLimit{}, fmt.Errorf("fetch pull requests by id: %w", err)
		}
		limit = latest(limit, resp.toDomain())
		found, l, err := c.toDomain(ctx, resp.Nodes)
		if err != nil {
			return nil, domain.RateLimit{}, err
		}
		prs = append(prs, found...)
		limit = latest(limit, l)
	}
	return prs, limit, nil
}

// toDomain maps pull request nodes into domain values, paging through the
// checks of any that has more than came with it. The rate limit is from the
// last checks page fetched, zero if none was.
func (c *Client) toDomain(ctx context.Context, nodes []pullRequestNode) ([]domain.PullRequest, domain.RateLimit, error) {
	var prs []domain.PullRequest
	var limit domain.RateLimit
	for _, n := range nodes {
		// Nodes that aren't pull requests, and IDs that didn't resolve,
		// decode as empty objects.
		if n.ID == "" {
			continue
		}
		pr, l, err := c.withAllChecks(ctx, n)
		if err != nil {
			return nil, domain.RateLimit{}, err
		}
		limit = latest(limit, l)
		prs = append(prs, pr)
	}
	return prs, limit, nil
}

// latest returns next, or prev when next is the zero value of a response that
// didn't report one.
func latest(prev, next domain.RateLimit) domain.RateLimit {
	if next == (domain.RateLimit{}) {
		return prev
	}
	return next
}

type pullRequestDetailResponse struct {
	Node *pullRequestNode
	rateLimitResponse
}

// ErrPullRequestNotFound means no pull request has the node ID asked for,
// usually because it was deleted or its repository became inaccessible.
var ErrPullRequestNotFound = errors.New("pull request not found")

// PullRequest fetches one pull request by node ID, with all of its checks,
// along with the rate-limit budget left afterwards.
func (c *Client) PullRequest(ctx context.Context, id string) (domain.PullRequest, domain.RateLimit, error) {
	var resp pullRequestDetailResponse
	if err := c.do(ctx, pullRequestDetailQuery, map[string]any{"id": id}, &resp); err != nil {
		return domain.PullRequest{}, domain.RateLimit{}, fmt.Errorf("load pull request: %w", err)
	}
	limit := resp.toDomain()
	if resp.Node == nil || resp.Node.ID == "" {
		return domain.PullRequest{}, limit, ErrPullRequestNotFound
	}
	pr, l, err := c.withAllChecks(ctx, *resp.Node)
	if err != nil {
		return domain.PullRequest{}, domain.RateLimit{}, err
	}
	if l != (domain.RateLimit{}) {
		limit = l
	}
	return pr, limit, nil
}

// withAllChecks maps a pull request node, fetching whatever checks didn't
// fit on its first page. The rate limit is zero when nothing more was fetched.
func (c *Client) withAllChecks(ctx context.Context, n pullRequestNode) (domain.PullRequest, domain.RateLimit, error) {
	pr := n.toDomain()
	rollup := n.rollup()
	if rollup == nil || !rollup.Contexts.PageInfo.HasNextPage {
		return pr, domain.RateLimit{}, nil
	}
	rest, limit, err := c.remainingChecks(ctx, n.headCommitID(), rollup.Contexts.PageInfo.EndCursor)
	if err != nil {
		return domain.PullRequest{}, domain.RateLimit{}, fmt.Errorf("checks for %s#%d: %w", pr.Repository.NameWithOwner, pr.Number, err)
	}
	pr.Checks = append(pr.Checks, rest...)
	return pr, limit, nil
}

const checkContextsQuery = `query CheckContexts($id: ID!, $after: String) {
  node(id: $id) {
    ... on Commit {
      statusCheckRollup {
        contexts(first: 100, after: $after) { ...CheckContextFields }
      }
    }
  }
  ...RateLimitFields
}` + checkContextFragment + rateLimitFragment

type checkContextsResponse struct {
	Node struct {
		StatusCheckRollup *statusCheckRollup
	}
	rateLimitResponse
}

// remainingChecks pages through a commit's checks after the first page, which
// came with the search.
func (c *Client) remainingChecks(ctx context.Context, commitID, after string) ([]domain.Check, domain.RateLimit, error) {
	var checks []domain.Check
	var limit domain.RateLimit
	for after != "" {
		var resp checkContextsResponse
		vars := map[string]any{"id": commitID, "after": after}
		if err := c.do(ctx, checkContextsQuery, vars, &resp); err != nil {
			return nil, domain.RateLimit{}, err
		}
		limit = resp.toDomain()
		rollup := resp.Node.StatusCheckRollup
		if rollup == nil {
			break
		}
		checks = append(checks, rollup.Contexts.toDomain()...)
		after = ""
		if rollup.Contexts.PageInfo.HasNextPage {
			after = rollup.Contexts.PageInfo.EndCursor
		}
	}
	return checks, limit, nil
}
