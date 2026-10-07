package store

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
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

var defaultTitlePattern = regexp.MustCompile(domain.DefaultTitlePattern)

// TitlePatternError is a title_pattern the config file sets but that doesn't
// compile. LoadConfig returns it with the rest of the config intact and the
// default pattern, so the app can run on and say what was wrong.
type TitlePatternError struct {
	Value string
	Err   error
}

func (e *TitlePatternError) Error() string {
	return fmt.Sprintf("%s: title_pattern %q doesn't compile (%v); using %s",
		ConfigFile, e.Value, e.Err, domain.DefaultTitlePattern)
}

func (e *TitlePatternError) Unwrap() error { return e.Err }

func parseTitlePattern(value string) (*regexp.Regexp, error) {
	if value == "" {
		return defaultTitlePattern, nil
	}
	re, err := regexp.Compile(value)
	if err != nil {
		return defaultTitlePattern, &TitlePatternError{Value: value, Err: err}
	}
	return re, nil
}

// Usable reports whether a config that LoadConfig returned with err can be
// used: err is nil, or it only names settings that fell back to their
// defaults.
func Usable(err error) bool {
	var intervalErr *RefreshIntervalError
	var patternErr *TitlePatternError
	return err == nil || errors.As(err, &intervalErr) || errors.As(err, &patternErr)
}
