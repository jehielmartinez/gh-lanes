// Package github is the only part of lanes that talks to the network. It
// speaks GraphQL to GitHub through go-gh and maps responses into domain values.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// ErrNotLoggedIn means gh holds no token for the host lanes would talk to.
var ErrNotLoggedIn = errors.New("not logged in")

// Client runs lanes' GraphQL queries against one GitHub host.
type Client struct {
	gql *api.GraphQLClient
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
	return &Client{gql: gql}, nil
}

// GitHub search never returns more than 1,000 results, so this many pages
// covers everything a search can produce.
const (
	searchPageSize = 50
	maxSearchPages = 1000 / searchPageSize
)

const searchPullRequestsQuery = `query SearchPullRequests($query: String!, $first: Int!, $after: String) {
  search(query: $query, type: ISSUE, first: $first, after: $after) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        id
        number
        title
        updatedAt
        repository { nameWithOwner }
      }
    }
  }
}`

type searchResponse struct {
	Search struct {
		PageInfo struct {
			HasNextPage bool
			EndCursor   string
		}
		Nodes []pullRequestNode
	}
}

type pullRequestNode struct {
	ID         string
	Number     int
	Title      string
	UpdatedAt  time.Time
	Repository struct {
		NameWithOwner string
	}
}

// SearchPullRequests returns every pull request the search string matches,
// paging through the results.
func (c *Client) SearchPullRequests(ctx context.Context, query string) ([]domain.PullRequest, error) {
	var prs []domain.PullRequest
	var after *string
	for range maxSearchPages {
		var resp searchResponse
		vars := map[string]any{"query": query, "first": searchPageSize, "after": after}
		if err := c.gql.DoWithContext(ctx, searchPullRequestsQuery, vars, &resp); err != nil {
			return nil, fmt.Errorf("search pull requests: %w", err)
		}
		for _, n := range resp.Search.Nodes {
			// Search nodes that aren't pull requests decode as empty objects.
			if n.ID == "" {
				continue
			}
			prs = append(prs, n.toDomain())
		}
		page := resp.Search.PageInfo
		if !page.HasNextPage || page.EndCursor == "" {
			break
		}
		after = &page.EndCursor
	}
	return prs, nil
}

func (n pullRequestNode) toDomain() domain.PullRequest {
	return domain.PullRequest{
		ID:         n.ID,
		Number:     n.Number,
		Title:      n.Title,
		Repository: n.Repository.NameWithOwner,
		UpdatedAt:  n.UpdatedAt,
	}
}
