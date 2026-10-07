package ui

import (
	"regexp"
	"slices"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// titlePatternLimit is the longest title pattern the editor takes.
const titlePatternLimit = 200

// groupings are the grouping picker's choices, in its order.
var groupings = []domain.Grouping{domain.GroupingNone, domain.GroupingOwner, domain.GroupingRepository, domain.GroupingTitlePattern}

// groupingLabel is how the grouping picker names a grouping.
func groupingLabel(g domain.Grouping) string {
	switch g {
	case domain.GroupingOwner:
		return "Owner"
	case domain.GroupingRepository:
		return "Repository"
	case domain.GroupingTitlePattern:
		return "Title pattern"
	}
	return "None"
}

// groupingChosenMsg is a grouping chosen in the grouping picker.
type groupingChosenMsg struct{ grouping domain.Grouping }

// groupingPicker is the picker of the grouping, with an editor for the title
// pattern opened from its Title pattern row.
type groupingPicker struct {
	list picker
	keys groupingPickerKeys
	form dialogKeys
	// editing is whether the title pattern editor has the keys.
	editing bool
	input   textinput.Model
	// compiled is the editor's pattern once it compiles. Until then it is
	// nil, the pattern can't be saved, and invalid says why.
	compiled *regexp.Regexp
	invalid  error
}

func newGroupingPicker(current domain.Grouping, titlePattern *regexp.Regexp) *groupingPicker {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = titlePatternLimit
	styles := input.Styles()
	styles.Cursor.Blink = false
	input.SetStyles(styles)
	gp := &groupingPicker{keys: newGroupingPickerKeys(), form: newDialogKeys(), input: input}
	gp.list = gp.listFor(current, titlePattern, slices.Index(groupings, current))
	return gp
}

// listFor is the list of groupings with current marked, titlePattern under
// Title pattern and the cursor on row cursor.
func (gp groupingPicker) listFor(current domain.Grouping, titlePattern *regexp.Regexp, cursor int) picker {
	items := make([]pickerItem, len(groupings))
	for i, g := range groupings {
		items[i] = pickerItem{label: groupingLabel(g)}
		if g == current {
			items[i].note = "(current)"
		}
		if g == domain.GroupingTitlePattern {
			items[i].detail = titlePattern.String()
		}
	}
	list := newPicker("Group by", items, cursor, func(i int) tea.Msg { return groupingChosenMsg{grouping: groupings[i]} })
	list.hints = []key.Binding{gp.keys.Edit}
	return *list
}

// withSettings shows the grouping and title pattern now in use, keeping the
// cursor where it is.
func (gp groupingPicker) withSettings(current domain.Grouping, titlePattern *regexp.Regexp) *groupingPicker {
	gp.list = gp.listFor(current, titlePattern, gp.list.cursor)
	return &gp
}

// typing reports whether keys are going into the pattern editor, where
// letters aren't commands.
func (gp groupingPicker) typing() bool { return gp.editing }

// update handles a key press or a paste. It returns the picker as it now
// stands, or nil once it has closed, the command a choice produced, and the
// title pattern to save, if one was.
func (gp groupingPicker) update(msg tea.Msg, titlePattern *regexp.Regexp) (*groupingPicker, tea.Cmd, *regexp.Regexp) {
	if !gp.editing {
		press, ok := msg.(tea.KeyPressMsg)
		if !ok {
			return &gp, nil, nil
		}
		if key.Matches(press, gp.keys.Edit) && groupings[gp.list.cursor] == domain.GroupingTitlePattern {
			return gp.edit(titlePattern.String()), nil, nil
		}
		list, cmd := gp.list.update(press)
		if list == nil {
			return nil, cmd, nil
		}
		gp.list = *list
		return &gp, cmd, nil
	}
	if press, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(press, gp.form.Cancel):
			return gp.stopEditing(), nil, nil
		case key.Matches(press, gp.form.Save):
			if gp.compiled == nil {
				return &gp, nil, nil
			}
			return gp.stopEditing(), nil, gp.compiled
		}
	}
	gp.input, _ = gp.input.Update(msg)
	return gp.validated(), nil, nil
}

func (gp groupingPicker) edit(pattern string) *groupingPicker {
	gp.editing = true
	gp.input.SetValue(pattern)
	gp.input.CursorEnd()
	gp.input.Focus()
	return gp.validated()
}

func (gp groupingPicker) validated() *groupingPicker {
	gp.compiled, gp.invalid = domain.CompileTitlePattern(gp.input.Value())
	return &gp
}

func (gp groupingPicker) stopEditing() *groupingPicker {
	gp.editing = false
	gp.input.Blur()
	return &gp
}

func (gp groupingPicker) view(t theme, h help.Model) string {
	if !gp.editing {
		return gp.list.view(t, h)
	}
	verdict := lipgloss.NewStyle().Foreground(t.success).Render("valid pattern")
	if gp.invalid != nil {
		verdict = lipgloss.NewStyle().Foreground(t.errText).Render(oneLine(capitalised(gp.invalid.Error())))
	}
	rows := []string{
		lipgloss.NewStyle().Bold(true).Foreground(t.text).Render("Title pattern"),
		"",
		gp.input.View(),
		verdict,
		"",
		h.ShortHelpView([]key.Binding{gp.form.Save, gp.form.Cancel}),
	}
	return modal(t, rows)
}

// capitalised is s with its first letter upper case, as a sentence starts.
func capitalised(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}

// openGroupingPicker opens the grouping picker. It stays shut while the
// config couldn't be read, since saving would overwrite it.
func (m Model) openGroupingPicker() Model {
	if m.storeReady {
		m.groupingPicker = newGroupingPicker(m.grouping, m.titlePattern)
	}
	return m
}

// groupingPickerUpdate hands a message to the grouping picker, and applies
// and saves the title pattern it saved.
func (m Model) groupingPickerUpdate(msg tea.Msg) (Model, tea.Cmd) {
	gp, cmd, pattern := m.groupingPicker.update(msg, m.titlePattern)
	m.groupingPicker = gp
	if pattern == nil {
		return m, cmd
	}
	source := pattern.String()
	m.titlePattern, m.titlePatternEdit = pattern, &source
	m = m.rebuild()
	m.groupingPicker = m.groupingPicker.withSettings(m.grouping, m.titlePattern)
	return m.saveConfig()
}

// chooseGrouping applies the grouping to every tab and saves it.
func (m Model) chooseGrouping(g domain.Grouping) (Model, tea.Cmd) {
	if !m.storeReady {
		return m, nil
	}
	m.grouping, m.groupingEdit = g, &g
	return m.rebuild().saveConfig()
}
