package ui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

const tagNameLimit = 40

type managerMode int

const (
	listMode managerMode = iota
	nameMode
	colorMode
	deleteMode
)

// tagEdit is a change the tag manager made: the tags as they now stand and,
// when pull requests changed lane, the new assignments.
type tagEdit struct {
	tags        []domain.Tag
	assignments map[string]string
}

// tagManager is the screen where tags are created, renamed, recoloured,
// reordered, deleted and marked terminal. Its rows are the lanes: Untagged
// first, which can't be changed, then one per tag.
type tagManager struct {
	keys tagManagerKeys
	form dialogKeys
	mode managerMode
	// cursor is the selected row; row i > 0 is tags[i-1].
	cursor int
	input  textinput.Model
	// editing is the ID of the tag being renamed, recoloured or deleted. It is
	// empty while a new tag is made.
	editing string
	// newName is the new tag's name while its colour is chosen.
	newName string
	// color indexes domain.TagColors.
	color int
	// note says why the last key did nothing, or what was wrong with a name.
	note string
}

func newTagManager(cursor int) *tagManager {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = tagNameLimit
	styles := input.Styles()
	styles.Cursor.Blink = false
	input.SetStyles(styles)
	return &tagManager{keys: newTagManagerKeys(), form: newDialogKeys(), cursor: cursor, input: input}
}

// typing reports whether keys are going into a text field, where letters
// aren't commands.
func (tm tagManager) typing() bool { return tm.mode == nameMode }

// update handles a message while the manager is open. It returns the manager as
// it now stands, or nil once it has closed, and any edit to apply.
func (tm tagManager) update(msg tea.Msg, tags []domain.Tag, assignments map[string]string) (*tagManager, *tagEdit) {
	tm.cursor = min(tm.cursor, len(tags))
	if paste, ok := msg.(tea.PasteMsg); ok && tm.mode == nameMode {
		tm.input, _ = tm.input.Update(paste)
		return &tm, nil
	}
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return &tm, nil
	}
	switch tm.mode {
	case nameMode:
		return tm.nameKey(press, tags)
	case colorMode:
		return tm.colorKey(press, tags, assignments)
	case deleteMode:
		return tm.deleteKey(press, tags, assignments)
	}
	return tm.listKey(press, tags)
}

func (tm tagManager) listKey(msg tea.KeyPressMsg, tags []domain.Tag) (*tagManager, *tagEdit) {
	tm.note = ""
	k := tm.keys
	switch {
	case key.Matches(msg, k.Close):
		return nil, nil
	case key.Matches(msg, k.Up):
		tm.cursor = max(0, tm.cursor-1)
		return &tm, nil
	case key.Matches(msg, k.Down):
		tm.cursor = min(len(tags), tm.cursor+1)
		return &tm, nil
	case key.Matches(msg, k.New):
		tm.editing = ""
		return tm.askName(""), nil
	case !key.Matches(msg, k.Rename, k.Color, k.MoveUp, k.MoveDown, k.Terminal, k.Delete):
		return &tm, nil
	case tm.cursor == 0:
		tm.note = board.UntaggedName + " can't be changed."
		return &tm, nil
	}

	tag := tags[tm.cursor-1]
	tm.editing = tag.ID
	switch {
	case key.Matches(msg, k.Rename):
		return tm.askName(tag.Name), nil
	case key.Matches(msg, k.Color):
		tm.mode = colorMode
		tm.color = max(0, slices.IndexFunc(domain.TagColors, func(c string) bool { return strings.EqualFold(c, tag.Color) }))
		return &tm, nil
	case key.Matches(msg, k.Delete):
		tm.mode = deleteMode
		return &tm, nil
	case key.Matches(msg, k.Terminal):
		next := board.UpdateTag(tags, tag.ID, func(t domain.Tag) domain.Tag {
			t.Terminal = !t.Terminal
			return t
		})
		return &tm, &tagEdit{tags: next}
	}
	delta := 1
	if key.Matches(msg, k.MoveUp) {
		delta = -1
	}
	next, at := board.MoveTag(tags, tag.ID, delta)
	if at+1 == tm.cursor {
		return &tm, nil
	}
	tm.cursor = at + 1
	return &tm, &tagEdit{tags: next}
}

func (tm tagManager) askName(current string) *tagManager {
	tm.mode = nameMode
	tm.input.SetValue(current)
	tm.input.CursorEnd()
	tm.input.Focus()
	return &tm
}

func (tm tagManager) nameKey(msg tea.KeyPressMsg, tags []domain.Tag) (*tagManager, *tagEdit) {
	switch {
	case key.Matches(msg, tm.form.Cancel):
		return tm.backToList(), nil
	case key.Matches(msg, tm.form.Save):
		name, err := board.CheckTagName(tags, tm.input.Value(), tm.editing)
		if err != nil {
			tm.note = err.Error()
			return &tm, nil
		}
		if tm.editing == "" {
			tm.newName = name
			tm.mode = colorMode
			tm.note = ""
			tm.color = slices.Index(domain.TagColors, board.UnusedColor(tags))
			return &tm, nil
		}
		next := board.UpdateTag(tags, tm.editing, func(t domain.Tag) domain.Tag {
			t.Name = name
			return t
		})
		return tm.backToList(), &tagEdit{tags: next}
	}
	tm.input, _ = tm.input.Update(msg)
	return &tm, nil
}

func (tm tagManager) colorKey(msg tea.KeyPressMsg, tags []domain.Tag, assignments map[string]string) (*tagManager, *tagEdit) {
	n := len(domain.TagColors)
	switch {
	case key.Matches(msg, tm.form.Cancel):
		return tm.backToList(), nil
	case key.Matches(msg, tm.form.Prev):
		tm.color = (tm.color + n - 1) % n
	case key.Matches(msg, tm.form.Next):
		tm.color = (tm.color + 1) % n
	case key.Matches(msg, tm.form.Save):
		color := domain.TagColors[tm.color]
		if tm.editing == "" {
			next := board.AddTag(tags, assignments, tm.newName, color)
			tm.cursor = len(next)
			return tm.backToList(), &tagEdit{tags: next}
		}
		next := board.UpdateTag(tags, tm.editing, func(t domain.Tag) domain.Tag {
			t.Color = color
			return t
		})
		return tm.backToList(), &tagEdit{tags: next}
	}
	return &tm, nil
}

func (tm tagManager) deleteKey(msg tea.KeyPressMsg, tags []domain.Tag, assignments map[string]string) (*tagManager, *tagEdit) {
	switch {
	case key.Matches(msg, tm.form.Decline):
		return tm.backToList(), nil
	case key.Matches(msg, tm.form.Confirm):
		nextTags, nextAssignments := board.DeleteTag(tags, assignments, tm.editing)
		tm.cursor = min(tm.cursor, len(nextTags))
		edit := &tagEdit{tags: nextTags}
		if len(nextAssignments) != len(assignments) {
			edit.assignments = nextAssignments
		}
		return tm.backToList(), edit
	}
	return &tm, nil
}

func (tm tagManager) backToList() *tagManager {
	tm.mode = listMode
	tm.note = ""
	tm.input.Blur()
	return &tm
}

// view draws the manager for the board's lanes, which are Untagged and then
// the tags in order.
func (tm tagManager) view(t theme, h help.Model, lanes []board.Lane) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(t.text)
	muted := lipgloss.NewStyle().Foreground(t.muted)
	var rows []string
	var bindings []key.Binding
	editing := laneOf(lanes, tm.editing)
	switch tm.mode {
	case nameMode:
		heading := "New tag"
		if tm.editing != "" {
			heading = "Rename " + oneLine(editing.Tag.Name)
		}
		rows = []string{title.Render(heading), "", tm.input.View()}
		bindings = []key.Binding{tm.form.Save, tm.form.Cancel}
	case colorMode:
		name := tm.newName
		if tm.editing != "" {
			name = editing.Tag.Name
		}
		rows = []string{title.Render("Color for " + oneLine(name)), "", tm.swatches(), muted.Render(domain.TagColors[tm.color])}
		bindings = []key.Binding{tm.form.Prev, tm.form.Next, tm.form.Save, tm.form.Cancel}
	case deleteMode:
		rows = []string{title.Render("Delete " + oneLine(editing.Tag.Name) + "?"), "", muted.Render(movesToUntagged(len(editing.PullRequests)))}
		bindings = []key.Binding{tm.form.Confirm, tm.form.Decline}
	default:
		rows = append([]string{title.Render("Tags"), ""}, tm.listRows(t, lanes)...)
		bindings = tm.keys.ShortHelp()
	}
	if tm.note != "" {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(t.errText).Render(tm.note))
	}
	rows = append(rows, "", h.ShortHelpView(bindings))
	return modal(t, rows)
}

func (tm tagManager) listRows(t theme, lanes []board.Lane) []string {
	nameWidth := 0
	for _, lane := range lanes {
		nameWidth = max(nameWidth, lipgloss.Width(oneLine(lane.Tag.Name)))
	}
	rows := make([]string, 0, len(lanes))
	for i, lane := range lanes {
		marker, style := "  ", lipgloss.NewStyle().Foreground(t.text)
		if i == tm.cursor {
			marker, style = "› ", style.Bold(true).Foreground(t.accent)
		}
		swatch := lipgloss.NewStyle().Foreground(laneColor(t, lane)).Render("●")
		name := style.Render(oneLine(lane.Tag.Name))
		name += strings.Repeat(" ", nameWidth-lipgloss.Width(oneLine(lane.Tag.Name)))
		row := marker + swatch + " " + name + "  " + lipgloss.NewStyle().Foreground(t.muted).Render(fmt.Sprint(len(lane.PullRequests)))
		if lane.Tag.Terminal {
			row += "  " + lipgloss.NewStyle().Foreground(t.muted).Render("terminal")
		}
		rows = append(rows, row)
	}
	return rows
}

func (tm tagManager) swatches() string {
	parts := make([]string, len(domain.TagColors))
	for i, c := range domain.TagColors {
		dot := lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("●")
		if i == tm.color {
			parts[i] = "[" + dot + "]"
		} else {
			parts[i] = " " + dot + " "
		}
	}
	return strings.Join(parts, "")
}

func movesToUntagged(n int) string {
	switch n {
	case 0:
		return "No pull requests are in it."
	case 1:
		return "Its pull request moves to " + board.UntaggedName + "."
	}
	return fmt.Sprintf("Its %d pull requests move to %s.", n, board.UntaggedName)
}

func laneOf(lanes []board.Lane, tagID string) board.Lane {
	i := slices.IndexFunc(lanes, func(l board.Lane) bool { return l.Tag.ID == tagID })
	if i < 0 {
		return board.Lane{}
	}
	return lanes[i]
}
