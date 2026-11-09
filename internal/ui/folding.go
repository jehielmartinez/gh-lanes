package ui

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// foldable is a heading in the modal that folds: a section, a review, a
// review thread, or a <details> block in markdown.
type foldable struct {
	key string
	// line is the content line the heading is drawn on.
	line int
	// open is whether it was drawn unfolded.
	open bool
}

// The keys foldables go by. Reviews and threads are keyed by node ID, so a
// fold survives a refresh that adds to the timeline.
func sectionFold(s section) string { return "section:" + s.key() }
func reviewFold(id string) string  { return "review:" + id }
func threadFold(id string) string  { return "thread:" + id }

// detailsFold keys the nth <details> block in the markdown of host: a
// review's fold key, a comment's, or the description.
func detailsFold(host string, n int) string { return host + "/details:" + strconv.Itoa(n) }

// folding is how the viewer has left the modal's reviews and threads.
type folding struct {
	expandResolved bool
	// folds is the viewer's own choice for each review and thread they
	// folded or unfolded. Anything else is drawn as its default.
	folds map[string]bool
	// highlight is the key of the heading the highlight is on, or empty.
	highlight string
}

func (d detail) folding() folding {
	return folding{expandResolved: d.expandResolved, folds: d.folds, highlight: d.highlight}
}

// isOpen is whether the foldable under key is unfolded, def unless the viewer
// chose otherwise.
func (f folding) isOpen(key string, def bool) bool {
	if open, ok := f.folds[key]; ok {
		return open
	}
	return def
}

// chosen is whether the viewer folded or unfolded key themselves.
func (f folding) chosen(key string) bool {
	_, ok := f.folds[key]
	return ok
}

// fold folds the heading if it is open and unfolds it if not.
func (d detail) fold(f foldable) detail {
	for s := range sectionCount {
		if f.key == sectionFold(s) {
			return d.toggle(s)
		}
	}
	// A detail is copied by value, so its folds are replaced, never changed
	// in place.
	folds := maps.Clone(d.folds)
	if folds == nil {
		folds = map[string]bool{}
	}
	folds[f.key] = !f.open
	d.folds = folds
	return d
}

// toggleResolved shows or hides every resolved thread, dropping the folds the
// viewer chose for single threads.
func (d detail) toggleResolved() detail {
	d.expandResolved = !d.expandResolved
	d.folds = maps.Clone(d.folds)
	maps.DeleteFunc(d.folds, func(key string, _ bool) bool { return strings.HasPrefix(key, "thread:") })
	return d
}

// foldHighlighted folds or unfolds the highlighted heading, if one is drawn.
func (d detail) foldHighlighted() detail {
	if i := d.highlighted(); i >= 0 {
		return d.fold(d.foldables[i])
	}
	return d
}

// highlighted is the index of the highlighted heading among those drawn, or
// -1.
func (d detail) highlighted() int {
	if d.highlight == "" {
		return -1
	}
	return slices.IndexFunc(d.foldables, func(f foldable) bool { return f.key == d.highlight })
}

// moveHighlight moves the highlight step headings on, wrapping around, and
// scrolls it into view. With nothing highlighted it starts from the first
// heading on screen going forward, or the last going back.
func (d detail) moveHighlight(step int) detail {
	n := len(d.foldables)
	if n == 0 {
		return d
	}
	top, bottom := d.viewport.YOffset(), d.viewport.YOffset()+d.viewport.Height()
	i := d.highlighted()
	switch {
	case i >= 0:
		i = (i + step + n) % n
	case step > 0:
		i = max(slices.IndexFunc(d.foldables, func(f foldable) bool { return f.line >= top }), 0)
	default:
		i = n - 1
		for j, f := range d.foldables {
			if f.line < bottom {
				i = j
			}
		}
	}
	f := d.foldables[i]
	d.highlight = f.key
	// A heading scrolled to sits near the top, so what it folds shows
	// beneath it.
	if f.line < top || f.line >= bottom {
		d.viewport.SetYOffset(max(f.line-2, 0))
	}
	return d
}

// highlightIf draws s as the highlighted heading's label when on is set.
func highlightIf(on bool, s string) string {
	if !on {
		return s
	}
	return lipgloss.NewStyle().Reverse(true).Bold(true).Render(s)
}
