// Package links finds the URLs in a pull request and records where they are
// drawn, so a click on the screen can be turned back into the URL under it.
package links

import (
	"errors"
	"net/url"
	"strings"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/markdown"
)

// ErrNotOpenable is the error for a URL Openable rejects.
var ErrNotOpenable = errors.New("only http and https links can be opened")

// Openable reports whether a URL may be handed to the browser: an absolute
// http or https URL with a host. PR content is untrusted, so anything else,
// such as javascript: or file:, is never opened.
func Openable(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// extractWidth is the width Extract lays text out at. Any width finds the
// same URLs, since a link broken across lines keeps its URL on every piece.
const extractWidth = 80

// Extract returns the openable URLs in markdown text, markdown links and bare
// URLs alike, once each in the order they first appear. It reads the text the
// way the markdown renderer draws it, so what it finds is what is clickable.
func Extract(text string) []string {
	var urls []string
	seen := map[string]bool{}
	for _, line := range markdown.Render(text, extractWidth) {
		for _, sp := range line {
			if sp.URL != "" && !seen[sp.URL] && Openable(sp.URL) {
				seen[sp.URL] = true
				urls = append(urls, sp.URL)
			}
		}
	}
	return urls
}

// Source is the part of a pull request a link was found in.
type Source int

// The sources a link can come from.
const (
	FromCheck Source = iota
	FromDescription
	FromComment
	FromReview
	FromThread
)

// Link is one URL in a pull request and where it came from.
type Link struct {
	URL    string
	Source Source
	// Where names the check or the thread's file:line; empty otherwise.
	Where string
	// Author is the login that wrote the text, "" for a deleted account, and
	// empty for checks.
	Author string
}

// Collect lists every openable URL in a pull request in the order the detail
// modal shows them: checks, the description, then the conversation timeline.
func Collect(pr domain.PullRequest) []Link {
	var out []Link
	for _, ch := range pr.Checks {
		if Openable(ch.DetailsURL) {
			out = append(out, Link{URL: ch.DetailsURL, Source: FromCheck, Where: ch.Name})
		}
	}
	add := func(text string, source Source, where, author string) {
		for _, u := range Extract(text) {
			out = append(out, Link{URL: u, Source: source, Where: where, Author: author})
		}
	}
	add(pr.Conversation.Body, FromDescription, "", pr.Author)
	for _, e := range pr.Conversation.Timeline() {
		switch {
		case e.Comment != nil:
			add(e.Comment.Body, FromComment, "", e.Comment.Author)
		case e.Review != nil:
			add(e.Review.Body, FromReview, "", e.Review.Author)
		case e.Thread != nil:
			for _, c := range e.Thread.Comments {
				add(c.Body, FromThread, e.Thread.Location(), c.Author)
			}
		}
	}
	return out
}

// Region is a run of cells on one line where part of a link is drawn: columns
// Start up to but not including End. A link wrapped over several lines has a
// region on each.
type Region struct {
	Line       int
	Start, End int
	URL        string
}

// Map is where every link on a page is drawn.
type Map []Region

// At returns the URL drawn at a line and column, if any.
func (m Map) At(line, col int) (string, bool) {
	for _, r := range m {
		if r.Line == line && col >= r.Start && col < r.End {
			return r.URL, true
		}
	}
	return "", false
}

// Indent returns the map with every region moved n columns right, for a page
// drawn behind a prefix.
func (m Map) Indent(n int) Map {
	out := make(Map, len(m))
	for i, r := range m {
		r.Start += n
		r.End += n
		out[i] = r
	}
	return out
}

// Shift returns the map moved down by n lines, for a page placed below
// another.
func (m Map) Shift(n int) Map {
	out := make(Map, len(m))
	for i, r := range m {
		r.Line += n
		out[i] = r
	}
	return out
}
