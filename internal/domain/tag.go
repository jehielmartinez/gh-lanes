package domain

// Tag is a lane the user defines locally. It never reaches GitHub.
type Tag struct {
	// ID is stable across renames; assignments reference it, never the name.
	ID    string
	Name  string
	Color string
	// Terminal marks a lane that holds finished work, such as Done.
	Terminal bool
}

// TagColors is the palette a tag's colour is chosen from. The default tags
// take the first ones, so a new tag starts on a colour none of them use.
var TagColors = []string{
	"#3B82F6",
	"#A855F7",
	"#F59E0B",
	"#10B981",
	"#6B7280",
	"#EF4444",
	"#EC4899",
	"#06B6D4",
	"#84CC16",
	"#F97316",
}
