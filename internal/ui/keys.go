package ui

import "charm.land/bubbles/v2/key"

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
	Quit      key.Binding
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
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.LaneLeft, k.LaneRight, k.CardUp, k.CardDown, k.MoveLeft, k.MoveRight, k.MoveTo, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
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
