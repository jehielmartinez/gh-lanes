package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
)

// keyMap is the single source of every binding; the help footer is generated
// from it.
type keyMap struct {
	Open    key.Binding
	Refresh key.Binding
	Quit    key.Binding
	Modal   modalKeys
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
		Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
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

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
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
