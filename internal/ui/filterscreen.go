package ui

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// filterScreen is the modal where owners and repositories are excluded from
// or included in every tab. Its rows are worked out afresh from the model each
// time, so they follow refreshes and config reloads while it is open.
type filterScreen struct {
	keys   filterKeys
	cursor int
	// expanded says, by lowercased login, whether each owner the screen has
	// listed shows its repositories.
	expanded map[string]bool
	// viewerErr is why the viewer's own account couldn't be loaded.
	viewerErr error
}

// filterRow is one row of the filter screen: an owner, or, when repo is set,
// one of its repositories.
type filterRow struct {
	owner board.Owner
	repo  *board.Repository
}

// filterToggle is the check the filter screen asks to flip: a repository's
// when repo is set, otherwise the owner's.
type filterToggle struct {
	owner, repo string
}

// viewerMsg is the result of asking GitHub which account lanes is
// authenticated as.
type viewerMsg struct {
	login string
	err   error
}

// withOwners returns the screen with every owner it hasn't listed before
// expanded if it is partial and collapsed otherwise. An owner keeps how it was
// first shown, so toggling it doesn't fold it away.
func (fs filterScreen) withOwners(owners []board.Owner) filterScreen {
	expanded := maps.Clone(fs.expanded)
	if expanded == nil {
		expanded = map[string]bool{}
	}
	for _, o := range owners {
		if _, ok := expanded[strings.ToLower(o.Login)]; !ok {
			expanded[strings.ToLower(o.Login)] = o.Partial
		}
	}
	fs.expanded = expanded
	return fs
}

func (fs filterScreen) isExpanded(login string) bool {
	return fs.expanded[strings.ToLower(login)]
}

func (fs filterScreen) withExpanded(login string, expanded bool) filterScreen {
	fs.expanded = maps.Clone(fs.expanded)
	fs.expanded[strings.ToLower(login)] = expanded
	return fs
}

// rows lists the owners, each followed by its repositories when expanded.
func (fs filterScreen) rows(owners []board.Owner) []filterRow {
	var rows []filterRow
	for _, o := range owners {
		rows = append(rows, filterRow{owner: o})
		if fs.isExpanded(o.Login) {
			for i := range o.Repositories {
				rows = append(rows, filterRow{owner: o, repo: &o.Repositories[i]})
			}
		}
	}
	return rows
}

// update handles a key press while the screen is open. It returns the screen
// as it now stands, or nil once it has closed, and the check to flip, if any.
func (fs filterScreen) update(msg tea.KeyPressMsg, owners []board.Owner) (*filterScreen, *filterToggle) {
	rows := fs.rows(owners)
	fs.cursor = max(0, min(fs.cursor, len(rows)-1))
	if key.Matches(msg, fs.keys.Close) {
		return nil, nil
	}
	if len(rows) == 0 {
		return &fs, nil
	}
	row := rows[fs.cursor]
	switch {
	case key.Matches(msg, fs.keys.Up):
		fs.cursor = max(0, fs.cursor-1)
	case key.Matches(msg, fs.keys.Down):
		fs.cursor = min(len(rows)-1, fs.cursor+1)
	case key.Matches(msg, fs.keys.Toggle) && row.repo != nil:
		return &fs, &filterToggle{owner: row.owner.Login, repo: row.repo.NameWithOwner}
	case key.Matches(msg, fs.keys.Toggle):
		return &fs, &filterToggle{owner: row.owner.Login}
	case row.repo != nil && key.Matches(msg, fs.keys.Collapse):
		fs = fs.withExpanded(row.owner.Login, false)
		fs.cursor = slices.IndexFunc(rows, func(r filterRow) bool { return r.repo == nil && r.owner.Login == row.owner.Login })
	case row.repo != nil:
		// A repository has nothing to expand.
	case key.Matches(msg, fs.keys.Fold):
		fs = fs.withExpanded(row.owner.Login, !fs.isExpanded(row.owner.Login))
	case key.Matches(msg, fs.keys.Expand):
		fs = fs.withExpanded(row.owner.Login, true)
	case key.Matches(msg, fs.keys.Collapse):
		fs = fs.withExpanded(row.owner.Login, false)
	}
	return &fs, nil
}

// repoIndent sets a repository row in under its owner.
const repoIndent = "    "

func (fs filterScreen) view(t theme, h help.Model, owners []board.Owner) string {
	rows := []string{lipgloss.NewStyle().Bold(true).Foreground(t.text).Render("Filter"), ""}
	type line struct {
		indent, check, name string
		count               int
	}
	var lines []line
	nameWidth := 0
	for _, r := range fs.rows(owners) {
		l := line{check: "[x]", name: oneLine(r.owner.Login), count: r.owner.PullRequests}
		switch {
		case r.repo != nil:
			l = line{indent: repoIndent, check: "[x]", name: oneLine(r.repo.Name()), count: r.repo.PullRequests}
			if r.repo.Excluded {
				l.check = "[ ]"
			}
		case r.owner.Partial:
			l.check = "[~]"
		case r.owner.Excluded:
			l.check = "[ ]"
		}
		lines = append(lines, l)
		nameWidth = max(nameWidth, len(l.indent)+lipgloss.Width(l.name))
	}
	muted := lipgloss.NewStyle().Foreground(t.muted)
	for i, l := range lines {
		marker, style := "  ", lipgloss.NewStyle().Foreground(t.text)
		if i == fs.cursor {
			marker, style = "› ", style.Bold(true).Foreground(t.accent)
		}
		pad := strings.Repeat(" ", nameWidth-len(l.indent)-lipgloss.Width(l.name))
		rows = append(rows, marker+l.indent+style.Render(l.check+" "+l.name+pad)+"  "+muted.Render(fmt.Sprint(l.count)))
	}
	if fs.viewerErr != nil {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(t.errText).Render(oneLine(fs.viewerErr.Error())))
	}
	rows = append(rows, "", h.ShortHelpView(fs.keys.ShortHelp()))
	return modal(t, rows)
}

// openFilterScreen opens the filter screen and asks GitHub for the viewer's
// account, which it lists first. It stays shut while the config couldn't be
// read, since saving would overwrite it.
func (m Model) openFilterScreen() (Model, tea.Cmd) {
	if !m.storeReady {
		return m, nil
	}
	fs := filterScreen{keys: newFilterKeys()}.withOwners(m.owners)
	m.filterScreen = &fs
	gh := m.opts.GitHub
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		login, err := gh.Viewer(ctx)
		return viewerMsg{login: login, err: err}
	}
}

func (m Model) viewerFetched(msg viewerMsg) Model {
	if msg.err == nil {
		m.viewer = msg.login
	}
	if m.filterScreen != nil {
		fs := *m.filterScreen
		fs.viewerErr = msg.err
		m.filterScreen = &fs
	}
	return m.rebuild()
}

// filterScreenKey hands a key press to the filter screen, and applies and
// saves the toggle it asks for.
func (m Model) filterScreenKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var toggle *filterToggle
	m.filterScreen, toggle = m.filterScreen.update(msg, m.owners)
	switch {
	case toggle == nil:
		return m, nil
	case toggle.repo != "":
		m.filter = board.ToggleRepository(m.filter, toggle.repo)
	default:
		m.filter = board.ToggleOwner(m.filter, toggle.owner)
	}
	return m.rebuild().saveConfig()
}

// ownerRows are the filter screen's rows. Their counts cover the open pull
// requests of Board and Review requests as the searches returned them, so
// they include what the filter hides.
func (m Model) ownerRows() []board.Owner {
	open := slices.DeleteFunc(slices.Clone(m.prs), domain.PullRequest.Finished)
	return board.Owners(m.viewer, slices.Concat(open, m.requested), m.filter)
}
