package domain

import "time"

// RateLimit is the GraphQL rate-limit budget left after a request.
type RateLimit struct {
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// Low reports whether less than a tenth of the budget is left. A zero value
// is never low, since it means GitHub didn't say.
func (r RateLimit) Low() bool {
	return r.Limit > 0 && r.Remaining*10 < r.Limit
}
