package domain

import (
	"errors"
	"regexp"
)

// Grouping is the one global mode that clusters cards into groups. It only
// changes how cards are displayed, never which lane a pull request is in.
type Grouping string

// The groupings. None, the zero value, shows no groups.
const (
	GroupingNone         Grouping = ""
	GroupingOwner        Grouping = "owner"
	GroupingRepository   Grouping = "repository"
	GroupingTitlePattern Grouping = "title_pattern"
)

// DefaultTitlePattern is the title pattern used when the config sets none,
// or one that doesn't compile: a Jira-style key such as SUP-1234.
const DefaultTitlePattern = `[A-Z][A-Z0-9]+-\d+`

// ErrEmptyTitlePattern is the error CompileTitlePattern gives an empty
// pattern, which would match nothing a group header could show.
var ErrEmptyTitlePattern = errors.New("the pattern is empty")

// CompileTitlePattern compiles a title pattern, refusing an empty one.
func CompileTitlePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, ErrEmptyTitlePattern
	}
	return regexp.Compile(pattern)
}
