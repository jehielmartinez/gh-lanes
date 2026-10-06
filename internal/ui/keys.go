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
	Open      key.Binding
	Archive   key.Binding
	Refresh   key.Binding
	Help      key.Binding
	Quit      key.Binding
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
}

func newKeyMap() keyMap {
	return keyMap{
		LaneLeft:  key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("←/h", "lane")),
		LaneRight: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("→/l", "lane")),
		CardUp:    key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "card")),
		CardDown:  key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "card")),
		MoveLeft:  key.NewBinding(key.WithKeys("H", "<"), key.WithHelp("H/<", "move left")),
		MoveRight: key.NewBinding(key.WithKeys("L", ">"), key.WithHelp("L/>", "move right")),
		MoveTo:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move to…")),
		Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Archive:   key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "archive")),
		Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Modal: modalKeys{
			Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
			Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
			PageUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("b/pgup", "page up")),
			PageDown: key.NewBinding(key.WithKeys("pgdown", "space", "f"), key.WithHelp("f/pgdn", "page down")),
			Top:      key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g/home", "top")),
			Bottom:   key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "bottom")),
			Close:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
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
		{k.MoveLeft, k.MoveRight, k.MoveTo, k.Archive},
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
	return []key.Binding{m.Up, m.Down, m.PageDown, m.Close, h.keys.Refresh, h.keys.Quit}
}

func (h modalHelp) FullHelp() [][]key.Binding {
	m := h.keys.Modal
	return [][]key.Binding{{m.Up, m.Down, m.PageUp, m.PageDown, m.Top, m.Bottom, m.Close, h.keys.Refresh, h.keys.Quit}}
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
