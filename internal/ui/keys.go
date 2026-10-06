package ui

import "charm.land/bubbles/v2/key"

// keyMap is the single source of every binding; the help footer is generated
// from it.
type keyMap struct {
	Refresh key.Binding
	Quit    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
