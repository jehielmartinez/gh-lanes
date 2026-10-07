package github

import (
	"context"
	"fmt"
)

const viewerQuery = `query Viewer {
  viewer { login }
}`

type viewerResponse struct {
	Viewer struct {
		Login string
	}
}

// Viewer returns the login of the account gh is authenticated as.
func (c *Client) Viewer(ctx context.Context) (string, error) {
	var resp viewerResponse
	if err := c.do(ctx, viewerQuery, nil, &resp); err != nil {
		return "", fmt.Errorf("load your account: %w", err)
	}
	return resp.Viewer.Login, nil
}

// organizationsPageSize is the most organizations GitHub returns in one page.
const organizationsPageSize = 100

const viewerOrganizationsQuery = `query ViewerOrganizations($first: Int!, $after: String) {
  viewer {
    organizations(first: $first, after: $after) {
      pageInfo { hasNextPage endCursor }
      nodes { login }
    }
  }
}`

type viewerOrganizationsResponse struct {
	Viewer struct {
		Organizations struct {
			PageInfo pageInfo
			Nodes    []struct{ Login string }
		}
	}
}

// ViewerOrganizations returns the logins of the organizations the viewer
// belongs to. An organization enforcing SAML SSO that gh's token isn't
// authorised for is left out by GitHub.
func (c *Client) ViewerOrganizations(ctx context.Context) ([]string, error) {
	var logins []string
	var after *string
	for {
		var resp viewerOrganizationsResponse
		vars := map[string]any{"first": organizationsPageSize, "after": after}
		if err := c.do(ctx, viewerOrganizationsQuery, vars, &resp); err != nil {
			return nil, fmt.Errorf("load your organizations: %w", err)
		}
		orgs := resp.Viewer.Organizations
		for _, n := range orgs.Nodes {
			logins = append(logins, n.Login)
		}
		if !orgs.PageInfo.HasNextPage || orgs.PageInfo.EndCursor == "" {
			return logins, nil
		}
		after = &orgs.PageInfo.EndCursor
	}
}
