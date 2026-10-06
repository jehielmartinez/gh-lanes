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
