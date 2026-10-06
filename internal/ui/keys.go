package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
)

// keyMap is the single source of every binding; the help footer is generated
// from it.
type keyMap struct {
	LaneLeft  key.Binding
	LaneRight key.Binding
	CardUp    key.Binding
	CardDown  key.Binding
	MoveLeft  key.Binding
	MoveRight key.Binding
	MoveTo    key.Binding
	Tags      key.Binding
	Open      key.Binding
	Archive   key.Binding
	// UpdateBranch, RebaseBranch and Draft act on the selected card, or on
	// the pull request in the modal.
	UpdateBranch key.Binding
	RebaseBranch key.Binding
	Draft        key.Binding
	Refresh      key.Binding
	Help         key.Binding
	Quit         key.Binding
	// Interrupt is the one way to quit while text is being typed, where q is
	// just a letter.
	Interrupt key.Binding
	Modal     modalKeys
}

// modalKeys are the bindings inside the detail modal.
type modalKeys struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Close    key.Binding
	// ExpandResolved shows or hides the comments of resolved review threads.
	ExpandResolved key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		LaneLeft:     key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("←/h", "lane")),
		LaneRight:    key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("→/l", "lane")),
		CardUp:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "card")),
		CardDown:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "card")),
		MoveLeft:     key.NewBinding(key.WithKeys("H", "<"), key.WithHelp("H/<", "move left")),
		MoveRight:    key.NewBinding(key.WithKeys("L", ">"), key.WithHelp("L/>", "move right")),
		MoveTo:       key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move to…")),
		Tags:         key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tags")),
		Open:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Archive:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "archive")),
		UpdateBranch: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "update branch")),
		RebaseBranch: key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "rebase branch")),
		Draft:        key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "draft")),
		Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Interrupt:    key.NewBinding(key.WithKeys("ctrl+c")),
		Modal: modalKeys{
			Up:             key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
			Down:           key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
			PageUp:         key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("b/pgup", "page up")),
			PageDown:       key.NewBinding(key.WithKeys("pgdown", "space", "f"), key.WithHelp("f/pgdn", "page down")),
			Top:            key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g/home", "top")),
			Bottom:         key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "bottom")),
			Close:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
			ExpandResolved: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "resolved threads")),
		},
	}
}

// withFullHelp returns the keymap with the help binding describing what
// pressing it will now do.
func (k keyMap) withFullHelp(showAll bool) keyMap {
	desc := "help"
	if showAll {
		desc = "close help"
	}
	k.Help.SetHelp("?", desc)
	return k
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.LaneLeft, k.LaneRight, k.CardUp, k.CardDown, k.Open, k.MoveTo, k.Help, k.Quit}
}

// FullHelp lists every binding on the board.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.LaneLeft, k.LaneRight, k.CardUp, k.CardDown, k.Open},
		{k.MoveLeft, k.MoveRight, k.MoveTo, k.Tags, k.Archive},
		{k.UpdateBranch, k.RebaseBranch, k.Draft},
		{k.Refresh, k.Help, k.Quit},
	}
}

// viewportKeys hands the modal's scroll bindings to the viewport. The
// viewport's own defaults also claim u, d, h and l, which belong to actions.
func (k modalKeys) viewportKeys() viewport.KeyMap {
	return viewport.KeyMap{Up: k.Up, Down: k.Down, PageUp: k.PageUp, PageDown: k.PageDown}
}

// modalHelp is the help footer while the modal is open.
type modalHelp struct{ keys keyMap }

func (h modalHelp) ShortHelp() []key.Binding {
	m := h.keys.Modal
	return []key.Binding{m.Up, m.Down, m.PageDown, m.ExpandResolved, m.Close, h.keys.UpdateBranch, h.keys.RebaseBranch, h.keys.Draft, h.keys.Refresh, h.keys.Quit}
}

func (h modalHelp) FullHelp() [][]key.Binding {
	m := h.keys.Modal
	return [][]key.Binding{
		{m.Up, m.Down, m.PageUp, m.PageDown, m.Top, m.Bottom, m.ExpandResolved, m.Close},
		{h.keys.UpdateBranch, h.keys.RebaseBranch, h.keys.Draft, h.keys.Refresh, h.keys.Quit},
	}
}

// pickerKeys are the bindings inside a picker.
type pickerKeys struct {
	Up     key.Binding
	Down   key.Binding
	Choose key.Binding
	Close  key.Binding
}

func newPickerKeys() pickerKeys {
	return pickerKeys{
		Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Choose: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "choose")),
		Close:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

func (k pickerKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Choose, k.Close}
}

func (k pickerKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// tagManagerKeys are the bindings in the tag manager's list.
type tagManagerKeys struct {
	Up       key.Binding
	Down     key.Binding
	New      key.Binding
	Rename   key.Binding
	Color    key.Binding
	MoveUp   key.Binding
	MoveDown key.Binding
	Terminal key.Binding
	Delete   key.Binding
	Close    key.Binding
}

func newTagManagerKeys() tagManagerKeys {
	return tagManagerKeys{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		New:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		Rename:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Color:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "color")),
		MoveUp:   key.NewBinding(key.WithKeys("K", "shift+up"), key.WithHelp("K", "move up")),
		MoveDown: key.NewBinding(key.WithKeys("J", "shift+down"), key.WithHelp("J", "move down")),
		Terminal: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "terminal")),
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Close:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

func (k tagManagerKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.New, k.Rename, k.Color, k.MoveUp, k.MoveDown, k.Terminal, k.Delete, k.Close}
}

func (k tagManagerKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down}, k.ShortHelp()}
}

// dialogKeys are the bindings while a tag's name or colour is being edited, or
// its deletion confirmed.
type dialogKeys struct {
	Prev    key.Binding
	Next    key.Binding
	Save    key.Binding
	Cancel  key.Binding
	Confirm key.Binding
	Decline key.Binding
}

func newDialogKeys() dialogKeys {
	return dialogKeys{
		Prev:    key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("←/h", "prev")),
		Next:    key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("→/l", "next")),
		Save:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Confirm: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "delete")),
		Decline: key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "keep")),
	}
}

// confirmKeys are the bindings inside a confirmation dialog.
type confirmKeys struct {
	Confirm key.Binding
	Cancel  key.Binding
}

func newConfirmKeys() confirmKeys {
	return confirmKeys{
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

func (k confirmKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Confirm, k.Cancel}
}
