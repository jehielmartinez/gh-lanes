package domain

// Archived is a pull request the user archived off the board. It stays off
// until it is reopened on GitHub.
type Archived struct {
	// ID is the pull request's node ID.
	ID string
	// Open is whether the pull request was open when last seen. A pull request
	// archived while open stays archived while it remains open; one that was
	// closed and shows up open again has been reopened.
	Open bool
	// Tag is the ID of the tag the pull request was archived from, so
	// unarchiving can put it back; empty for Untagged.
	Tag string
}
