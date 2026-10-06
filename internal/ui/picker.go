package ui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// pickerItem is one choice in a picker.
type pickerItem struct {
	label string
	// note is shown dimmed after the label, such as "(current)".
	note string
}

// picker is a small modal list: the user moves a cursor and chooses one item,
// or closes it. What a choice means is up to whoever opened it.
type picker struct {
	title  string
	items  []pickerItem
	cursor int
	keys   pickerKeys
	// choose turns the chosen index into the message the root model acts on.
	choose func(index int) tea.Msg
}

func newPicker(title string, items []pickerItem, cursor int, choose func(int) tea.Msg) *picker {
	return &picker{
		title:  title,
		items:  items,
		cursor: max(0, min(cursor, len(items)-1)),
		keys:   newPickerKeys(),
		choose: choose,
	}
}

// update handles a key press. It returns the picker as it now stands, or nil
// once it has closed, and the command a choice produced.
func (p picker) update(msg tea.KeyPressMsg) (*picker, tea.Cmd) {
	switch {
	case key.Matches(msg, p.keys.Up):
		p.cursor = max(0, p.cursor-1)
	case key.Matches(msg, p.keys.Down):
		p.cursor = min(len(p.items)-1, p.cursor+1)
	case key.Matches(msg, p.keys.Close):
		return nil, nil
	case key.Matches(msg, p.keys.Choose):
		if len(p.items) == 0 {
			return nil, nil
		}
		index, choose := p.cursor, p.choose
		return nil, func() tea.Msg { return choose(index) }
	}
	return &p, nil
}

func (p picker) view(t theme, h help.Model) string {
	rows := []string{lipgloss.NewStyle().Bold(true).Foreground(t.text).Render(p.title), ""}
	for i, item := range p.items {
		marker, style := "  ", lipgloss.NewStyle().Foreground(t.text)
		if i == p.cursor {
			marker, style = "› ", style.Bold(true).Foreground(t.accent)
		}
		row := marker + style.Render(item.label)
		if item.note != "" {
			row += " " + lipgloss.NewStyle().Foreground(t.muted).Render(item.note)
		}
		rows = append(rows, row)
	}
	rows = append(rows, "", h.ShortHelpView(p.keys.ShortHelp()))
	return modal(t, rows)
}

// modal frames rows as a dialog drawn over the board.
func modal(t theme, rows []string) string {
	return lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.accent).
		Render(strings.Join(rows, "\n"))
}
