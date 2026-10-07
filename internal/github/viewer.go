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
