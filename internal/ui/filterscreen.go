package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// filterScreen is the modal where owners are excluded from or included in
// every tab. Its rows are worked out afresh from the model each time, so they
// follow refreshes and config reloads while it is open.
type filterScreen struct {
	keys   filterKeys
	cursor int
	// viewerErr is why the viewer's own account couldn't be loaded.
	viewerErr error
	// organizationsErr is why the viewer's organizations couldn't be loaded.
	organizationsErr error
}

// viewerMsg is the result of asking GitHub which account lanes is
// authenticated as.
type viewerMsg struct {
	login string
	err   error
}

// organizationsMsg is the result of asking GitHub which organizations the
// viewer belongs to.
type organizationsMsg struct {
	logins []string
	err    error
}

// update handles a key press while the screen is open. It returns the screen
// as it now stands, or nil once it has closed, and the login of the owner to
// toggle, empty for none.
func (fs filterScreen) update(msg tea.KeyPressMsg, owners []board.Owner) (*filterScreen, string) {
	fs.cursor = max(0, min(fs.cursor, len(owners)-1))
	switch {
	case key.Matches(msg, fs.keys.Close):
		return nil, ""
	case key.Matches(msg, fs.keys.Up):
		fs.cursor = max(0, fs.cursor-1)
	case key.Matches(msg, fs.keys.Down):
		fs.cursor = max(0, min(len(owners)-1, fs.cursor+1))
	case key.Matches(msg, fs.keys.Toggle) && len(owners) > 0:
		return &fs, owners[fs.cursor].Login
	}
	return &fs, ""
}

func (fs filterScreen) view(t theme, h help.Model, owners []board.Owner) string {
	rows := []string{lipgloss.NewStyle().Bold(true).Foreground(t.text).Render("Filter"), ""}
	nameWidth := 0
	for _, o := range owners {
		nameWidth = max(nameWidth, lipgloss.Width(oneLine(o.Login)))
	}
	muted := lipgloss.NewStyle().Foreground(t.muted)
	for i, o := range owners {
		marker, style := "  ", lipgloss.NewStyle().Foreground(t.text)
		if i == fs.cursor {
			marker, style = "› ", style.Bold(true).Foreground(t.accent)
		}
		check := "[x]"
		if o.Excluded {
			check = "[ ]"
		}
		name := oneLine(o.Login)
		name += strings.Repeat(" ", nameWidth-lipgloss.Width(name))
		rows = append(rows, marker+style.Render(check+" "+name)+"  "+muted.Render(fmt.Sprint(o.PullRequests)))
	}
	errStyle := lipgloss.NewStyle().Foreground(t.errText)
	if fs.viewerErr != nil || fs.organizationsErr != nil {
		rows = append(rows, "")
	}
	for _, err := range []error{fs.viewerErr, fs.organizationsErr} {
		if err != nil {
			rows = append(rows, errStyle.Render(oneLine(err.Error())))
		}
	}
	rows = append(rows, "", h.ShortHelpView(fs.keys.ShortHelp()))
	return modal(t, rows)
}

// openFilterScreen opens the filter screen and asks GitHub for the viewer's
// account, which it lists first, and the viewer's organizations, which it
// lists even with no pull requests. It stays shut while the config couldn't be
// read, since saving would overwrite it.
func (m Model) openFilterScreen() (Model, tea.Cmd) {
	if !m.storeReady {
		return m, nil
	}
	m.filterScreen = &filterScreen{keys: newFilterKeys()}
	gh := m.opts.GitHub
	viewer := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		login, err := gh.Viewer(ctx)
		return viewerMsg{login: login, err: err}
	}
	organizations := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		logins, err := gh.ViewerOrganizations(ctx)
		return organizationsMsg{logins: logins, err: err}
	}
	return m, tea.Batch(viewer, organizations)
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

// organizationsFetched lists the viewer's organizations. A failed load keeps
// the organizations last loaded, if any.
func (m Model) organizationsFetched(msg organizationsMsg) Model {
	if msg.err == nil {
		m.organizations = msg.logins
	}
	if m.filterScreen != nil {
		fs := *m.filterScreen
		fs.organizationsErr = msg.err
		m.filterScreen = &fs
	}
	return m.rebuild()
}

// filterScreenKey hands a key press to the filter screen, and applies and
// saves the toggle it asks for.
func (m Model) filterScreenKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var owner string
	m.filterScreen, owner = m.filterScreen.update(msg, m.owners)
	if owner == "" {
		return m, nil
	}
	m.filter = board.ToggleOwner(m.filter, owner)
	return m.rebuild().saveConfig()
}

// ownerRows are the filter screen's rows. Their counts cover the open pull
// requests of Board and Review requests as the searches returned them, so
// they include what the filter hides.
func (m Model) ownerRows() []board.Owner {
	open := slices.DeleteFunc(slices.Clone(m.prs), domain.PullRequest.Finished)
	return board.Owners(m.viewer, m.organizations, slices.Concat(open, m.requested), m.filter)
}
