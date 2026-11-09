package domain

import (
	"fmt"
	"slices"
	"time"
)

// Conversation is a pull request's description and everything said on it.
type Conversation struct {
	Body     string
	Comments []Comment
	Reviews  []Review
	Threads  []Thread
}

// Comment is one comment, on the pull request itself or in a review thread.
type Comment struct {
	Author    string
	Body      string
	CreatedAt time.Time
}

// Thread is one review thread: comments anchored to a line of a file.
type Thread struct {
	ID   string
	Path string
	// Line is the line the thread is anchored to, or the line it was
	// anchored to before the code moved. It is zero for a comment on the
	// whole file.
	Line     int
	Resolved bool
	Comments []Comment
	// ReviewID is the review that opened the thread, empty when GitHub
	// doesn't say.
	ReviewID string
}

// StartedAt is when the thread's first comment was made.
func (t Thread) StartedAt() time.Time {
	if len(t.Comments) == 0 {
		return time.Time{}
	}
	return t.Comments[0].CreatedAt
}

// TimelineEntry is one item of the conversation timeline: exactly one of
// Comment, Review and Thread is set. A review entry also holds the threads
// it opened, oldest first, which then appear nowhere else in the timeline.
type TimelineEntry struct {
	At      time.Time
	Comment *Comment
	Review  *Review
	Thread  *Thread
	Threads []*Thread
}

// Timeline merges the conversation's comments, reviews and threads into one
// list, oldest first, each thread under the review that opened it. A review
// that only commented, said nothing and opened no thread is left out: GitHub
// creates one to carry each batch of replies, and those already appear in
// their threads.
func (c Conversation) Timeline() []TimelineEntry {
	opened := map[string][]*Thread{}
	for i, t := range c.Threads {
		if t.ReviewID != "" {
			opened[t.ReviewID] = append(opened[t.ReviewID], &c.Threads[i])
		}
	}
	var entries []TimelineEntry
	for i := range c.Comments {
		entries = append(entries, TimelineEntry{At: c.Comments[i].CreatedAt, Comment: &c.Comments[i]})
	}
	nested := map[*Thread]bool{}
	for i, r := range c.Reviews {
		threads := opened[r.ID]
		if r.State == ReviewStateCommented && r.Body == "" && len(threads) == 0 {
			continue
		}
		slices.SortStableFunc(threads, func(a, b *Thread) int { return a.StartedAt().Compare(b.StartedAt()) })
		for _, t := range threads {
			nested[t] = true
		}
		entries = append(entries, TimelineEntry{At: r.SubmittedAt, Review: &c.Reviews[i], Threads: threads})
	}
	for i := range c.Threads {
		if !nested[&c.Threads[i]] {
			entries = append(entries, TimelineEntry{At: c.Threads[i].StartedAt(), Thread: &c.Threads[i]})
		}
	}
	slices.SortStableFunc(entries, func(a, b TimelineEntry) int { return a.At.Compare(b.At) })
	return entries
}

// Location is the thread's file:line, or just the file for a comment on the
// whole file.
func (t Thread) Location() string {
	if t.Line == 0 {
		return t.Path
	}
	return fmt.Sprintf("%s:%d", t.Path, t.Line)
}
