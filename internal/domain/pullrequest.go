// Package domain holds the values the rest of lanes reasons about. It does no
// I/O and imports nothing from this module.
package domain

import "time"

// PullRequest is one pull request as the board sees it.
type PullRequest struct {
	// ID is the GraphQL node ID, the key for everything stored locally.
	ID         string
	Number     int
	Title      string
	Repository string
	UpdatedAt  time.Time
}
