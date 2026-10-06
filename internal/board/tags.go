package board

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// CheckTagName trims name and returns it, or an error saying why no tag can
// take it. except is the ID of the tag being renamed, which may keep its own
// name; it is empty for a new tag.
func CheckTagName(tags []domain.Tag, name, except string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", errors.New("a tag needs a name")
	}
	if strings.EqualFold(name, UntaggedName) {
		return "", fmt.Errorf("%q already exists", UntaggedName)
	}
	for _, t := range tags {
		if t.ID != except && strings.EqualFold(t.Name, name) {
			return "", fmt.Errorf("%q already exists", t.Name)
		}
	}
	return name, nil
}

// AddTag returns tags with a new tag at the end. Its ID is derived from the
// name and never changes after. No tag uses it, and no assignment refers to it,
// so a tag removed from the config by hand can't hand its pull requests to a
// new one.
func AddTag(tags []domain.Tag, assignments map[string]string, name, color string) []domain.Tag {
	return append(slices.Clone(tags), domain.Tag{ID: newTagID(tags, assignments, name), Name: name, Color: color})
}

func newTagID(tags []domain.Tag, assignments map[string]string, name string) string {
	referenced := map[string]bool{}
	for _, id := range assignments {
		referenced[id] = true
	}
	base := slug(name)
	taken := func(id string) bool {
		return referenced[id] || slices.ContainsFunc(tags, func(t domain.Tag) bool { return t.ID == id })
	}
	id := base
	for n := 2; taken(id); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	if b.Len() == 0 {
		return "tag"
	}
	return b.String()
}

// UnusedColor returns the first palette colour no tag uses, or the first
// colour when every one is taken.
func UnusedColor(tags []domain.Tag) string {
	for _, c := range domain.TagColors {
		if !slices.ContainsFunc(tags, func(t domain.Tag) bool { return strings.EqualFold(t.Color, c) }) {
			return c
		}
	}
	return domain.TagColors[0]
}

// UpdateTag returns tags with the tag of the given ID replaced by what change
// makes of it. The ID itself can't change.
func UpdateTag(tags []domain.Tag, id string, change func(domain.Tag) domain.Tag) []domain.Tag {
	next := slices.Clone(tags)
	for i, t := range next {
		if t.ID == id {
			next[i] = change(t)
			next[i].ID = id
		}
	}
	return next
}

// MoveTag returns tags with the tag of the given ID moved delta places, kept
// within the list, and the index it ends up at.
func MoveTag(tags []domain.Tag, id string, delta int) ([]domain.Tag, int) {
	from := slices.IndexFunc(tags, func(t domain.Tag) bool { return t.ID == id })
	if from < 0 {
		return tags, -1
	}
	to := max(0, min(from+delta, len(tags)-1))
	next := slices.Clone(tags)
	tag := next[from]
	next = slices.Delete(next, from, from+1)
	return slices.Insert(next, to, tag), to
}

// DeleteTag returns tags without the tag of the given ID, and assignments with
// every pull request that was in it moved to Untagged.
func DeleteTag(tags []domain.Tag, assignments map[string]string, id string) ([]domain.Tag, map[string]string) {
	nextTags := slices.DeleteFunc(slices.Clone(tags), func(t domain.Tag) bool { return t.ID == id })
	nextAssignments := make(map[string]string, len(assignments))
	for pr, tag := range assignments {
		if tag != id {
			nextAssignments[pr] = tag
		}
	}
	return nextTags, nextAssignments
}
