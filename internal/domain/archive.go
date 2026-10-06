package domain

// Archived is a pull request the user archived off the board or out of the
// review requests. It stays off until it leaves its origin's search and
// shows up there again.
type Archived struct {
	// ID is the pull request's node ID.
	ID string
	// Open is whether the pull request was in its origin's search when last
	// seen: open, for the board; requesting the viewer's review, for review
	// requests. One archived while in the search stays archived while it
	// remains there; one that was out of it and shows up again comes back.
	Open bool
	// Tag is the ID of the tag the pull request was archived from, so
	// unarchiving can put it back; empty for Untagged.
	Tag string
	// Origin is the tab the pull request was archived from, and the one
	// unarchiving returns it to.
	Origin Origin
}

// Origin is the tab an archived pull request came from.
type Origin string

const (
	// OriginBoard is the zero value, so entries saved before review requests
	// could be archived read as archived off the board.
	OriginBoard          Origin = ""
	OriginReviewRequests Origin = "review_requests"
)
