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
	// offset is the first row in view when the rows outgrow the screen.
	offset int
	// expanded says, by lowercased login, whether each owner the screen has
	// listed shows its repositories.
	expanded map[string]bool
	// viewerErr is why the viewer's own account couldn't be loaded.
	viewerErr error
	// organizationsErr is why the viewer's organizations couldn't be loaded.
	organizationsErr error
	// loading holds, by lowercased login, the owners whose repositories are
	// being loaded.
	loading map[string]string
	// repositoriesErr is why an owner's repositories couldn't be loaded.
	repositoriesErr error
}

// filterRow is one row of the filter screen: an owner, or, when repo is set,
// one of its repositories.
type filterRow struct {
	owner board.Owner
	repo  *board.Repository
}

// filterAction is what the filter screen asks the root model to do: load
// every repository of owner when load is set, or else flip a check, the
// repository's when repo is set and otherwise the owner's.
type filterAction struct {
	owner, repo string
	load        bool
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

// organizationsMsg is the result of asking GitHub which organizations the
// viewer belongs to.
type organizationsMsg struct {
	logins []string
	err    error
}

// update handles a key press while the screen is open, in a terminal
// termHeight rows tall. It returns the screen as it now stands, or nil once it
// has closed, and what it asks of the root model, if anything.
func (fs filterScreen) update(msg tea.KeyPressMsg, owners []board.Owner, termHeight int) (*filterScreen, *filterAction) {
	rows := fs.rows(owners)
	fs.cursor = max(0, min(fs.cursor, len(rows)-1))
	if key.Matches(msg, fs.keys.Close) {
		return nil, nil
	}
	if len(rows) == 0 {
		return &fs, nil
	}
	row := rows[fs.cursor]
	var action *filterAction
	switch {
	case key.Matches(msg, fs.keys.Up):
		fs.cursor = max(0, fs.cursor-1)
	case key.Matches(msg, fs.keys.Down):
		fs.cursor = min(len(rows)-1, fs.cursor+1)
	case key.Matches(msg, fs.keys.Toggle) && row.repo != nil:
		action = &filterAction{owner: row.owner.Login, repo: row.repo.NameWithOwner}
	case key.Matches(msg, fs.keys.Toggle):
		action = &filterAction{owner: row.owner.Login}
	case key.Matches(msg, fs.keys.Load):
		action = &filterAction{owner: row.owner.Login, load: true}
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
	if size := fs.listHeight(termHeight); size > 0 {
		fs.offset = window(fs.offset, fs.cursor, size, len(fs.rows(owners)))
	}
	return &fs, action
}

// filterChrome is how many lines the screen takes besides its rows and
// notices: the border, the title, the help line and the blank lines between.
const filterChrome = 6

// listHeight is how many rows fit in a terminal termHeight rows tall, or 0
// when the height isn't known and every row is shown.
func (fs filterScreen) listHeight(termHeight int) int {
	if termHeight == 0 {
		return 0
	}
	used := filterChrome
	if n := len(fs.notices()); n > 0 {
		used += 1 + n
	}
	return max(termHeight-used, 1)
}

// filterNotice is one line under the screen's rows: a load in flight, or why
// one failed.
type filterNotice struct {
	text string
	err  bool
}

func (fs filterScreen) notices() []filterNotice {
	var notices []filterNotice
	for _, lower := range slices.Sorted(maps.Keys(fs.loading)) {
		notices = append(notices, filterNotice{text: "Loading " + fs.loading[lower] + "'s repositories…"})
	}
	for _, err := range []error{fs.viewerErr, fs.organizationsErr, fs.repositoriesErr} {
		if err != nil {
			notices = append(notices, filterNotice{text: err.Error(), err: true})
		}
	}
	return notices
}

func (fs filterScreen) withLoading(login string, loading bool) filterScreen {
	fs.loading = maps.Clone(fs.loading)
	if fs.loading == nil {
		fs.loading = map[string]string{}
	}
	if loading {
		fs.loading[strings.ToLower(login)] = login
	} else {
		delete(fs.loading, strings.ToLower(login))
	}
	return fs
}

// repoIndent sets a repository row in under its owner.
const repoIndent = "    "

func (fs filterScreen) view(t theme, h help.Model, owners []board.Owner, termHeight int) string {
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
	first, last := 0, len(lines)
	if size := fs.listHeight(termHeight); size > 0 {
		first = window(fs.offset, fs.cursor, size, len(lines))
		last = min(last, first+size)
	}
	for i := first; i < last; i++ {
		l := lines[i]
		marker, style := "  ", lipgloss.NewStyle().Foreground(t.text)
		if i == fs.cursor {
			marker, style = "› ", style.Bold(true).Foreground(t.accent)
		}
		pad := strings.Repeat(" ", nameWidth-len(l.indent)-lipgloss.Width(l.name))
		rows = append(rows, marker+l.indent+style.Render(l.check+" "+l.name+pad)+"  "+muted.Render(fmt.Sprint(l.count)))
	}
	notices := fs.notices()
	if len(notices) > 0 {
		rows = append(rows, "")
	}
	for _, n := range notices {
		style := muted
		if n.err {
			style = lipgloss.NewStyle().Foreground(t.errText)
		}
		rows = append(rows, style.Render(oneLine(n.text)))
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
	fs := filterScreen{keys: newFilterKeys()}.withOwners(m.owners)
	m.filterScreen = &fs
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

// filterScreenKey hands a key press to the filter screen, and starts the load
// or applies and saves the toggle it asks for.
func (m Model) filterScreenKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	var action *filterAction
	m.filterScreen, action = m.filterScreen.update(msg, m.owners, m.height)
	switch {
	case action == nil:
		return m, nil
	case action.load:
		return m.loadOwnerRepositories(action.owner)
	case action.repo != "":
		m.filter = board.ToggleRepository(m.filter, action.repo)
	default:
		m.filter = board.ToggleOwner(m.filter, action.owner)
	}
	return m.rebuild().saveConfig()
}

// ownerRepositoriesMsg is the result of asking GitHub for every repository of
// an owner.
type ownerRepositoriesMsg struct {
	owner string
	repos []string
	err   error
}

// loadOwnerRepositories asks GitHub for every repository of owner, so one can
// be excluded before it has a pull request.
func (m Model) loadOwnerRepositories(owner string) (Model, tea.Cmd) {
	fs := m.filterScreen.withLoading(owner, true)
	m.filterScreen = &fs
	gh := m.opts.GitHub
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		repos, err := gh.OwnerRepositories(ctx, owner)
		return ownerRepositoriesMsg{owner: owner, repos: repos, err: err}
	}
}

// ownerRepositoriesFetched lists the owner's loaded repositories in place of
// any loaded before, and expands the owner to show them. A failed load keeps
// the rows as they were.
func (m Model) ownerRepositoriesFetched(msg ownerRepositoriesMsg) Model {
	if msg.err == nil {
		others := slices.DeleteFunc(slices.Clone(m.ownerRepositories), func(repo string) bool {
			owner, _, _ := strings.Cut(repo, "/")
			return strings.EqualFold(owner, msg.owner)
		})
		m.ownerRepositories = slices.Concat(others, msg.repos)
	}
	if m.filterScreen != nil {
		fs := m.filterScreen.withLoading(msg.owner, false)
		fs.repositoriesErr = msg.err
		if msg.err == nil {
			fs = fs.withExpanded(msg.owner, true)
		}
		m.filterScreen = &fs
	}
	return m.rebuild()
}

// ownerRows are the filter screen's rows. Their counts cover the open pull
// requests of Board and Review requests as the searches returned them, so
// they include what the filter hides.
func (m Model) ownerRows() []board.Owner {
	open := slices.DeleteFunc(slices.Clone(m.prs), domain.PullRequest.Finished)
	return board.Owners(m.viewer, m.organizations, m.ownerRepositories, slices.Concat(open, m.requested), m.filter)
}
