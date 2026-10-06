package store

import (
	"fmt"
	"time"
)

// DefaultRefreshInterval is how often the board refreshes when the config
// doesn't say.
const DefaultRefreshInterval = 60 * time.Second

// MinRefreshInterval is the shortest refresh interval accepted, so a typo
// can't spend the whole rate-limit budget.
const MinRefreshInterval = 10 * time.Second

// RefreshIntervalError is a refresh_interval the config file sets but that
// can't be used. LoadConfig returns it with the rest of the config intact and
// the default interval, so the app can run on and say what was wrong.
type RefreshIntervalError struct {
	Value string
}

func (e *RefreshIntervalError) Error() string {
	return fmt.Sprintf("%s: refresh_interval %q must be a duration of at least %s, like 60s or 5m; using %s",
		ConfigFile, e.Value, MinRefreshInterval, DefaultRefreshInterval)
}

func parseRefreshInterval(value string) (time.Duration, error) {
	if value == "" {
		return DefaultRefreshInterval, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < MinRefreshInterval {
		return DefaultRefreshInterval, &RefreshIntervalError{Value: value}
	}
	return d, nil
}
