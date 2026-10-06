// Package ui is the Bubble Tea front end: the root model, its views and the
// commands that reach the GitHub layer and the local files.
package ui

import (
	"context"
	"slices"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/store"
)

const boardSearch = "is:pr is:open author:@me archived:false"

const fetchTimeout = 30 * time.Second

// GitHub is what the UI needs from the GitHub layer.
type GitHub interface {
	SearchPullRequests(ctx context.Context, query string) ([]domain.PullRequest, error)
}

// Options are the boundaries the root model is given rather than reaching for
// itself, so tests can substitute every one of them.
type Options struct {
	GitHub GitHub
	// ConfigDir holds the config and state files.
	ConfigDir string
	// Now is the clock. Local times are shown in the location of the times it
	// returns.
	Now func() time.Time
}

// Model is the root model of the app.
type Model struct {
	opts  Options
	keys  keyMap
	help  help.Model
	theme theme

	width, height int

	loading bool
	prs     []domain.PullRequest
	err     error

	// storeReady is false until the config and state files have been read.
	// Nothing is written while it is false, so a file lanes couldn't read is
	// never overwritten.
	storeReady  bool
	storeErr    error
	tags        []domain.Tag
	assignments map[string]string

	// One state write is in flight at a time; a move made meanwhile queues
	// another, so writes land in the order the moves were made.
	saving     bool
	saveQueued bool
	saveErr    error
	quitting   bool

	lanes   []board.Lane
	focus   int
	cursors []int
	picker  *picker
}

// New returns the root model, ready to load the board when started.
func New(opts Options) Model {
	m := Model{
		opts:    opts,
		keys:    newKeyMap(),
		help:    help.New(),
		theme:   newTheme(true),
		loading: true,
	}
	return m.rebuild()
}

type pullRequestsMsg struct {
	prs []domain.PullRequest
	err error
}

type storeLoadedMsg struct {
	config store.Config
	state  store.State
	err    error
}

type stateSavedMsg struct{ err error }

// moveMsg asks for a pull request to be put in the lane of a tag; an empty
// tag ID is Untagged.
type moveMsg struct{ prID, tagID string }

func (m Model) fetchBoard() tea.Cmd {
	gh := m.opts.GitHub
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, err := gh.SearchPullRequests(ctx, boardSearch)
		return pullRequestsMsg{prs: prs, err: err}
	}
}

func (m Model) loadStore() tea.Cmd {
	dir := m.opts.ConfigDir
	return func() tea.Msg {
		cfg, err := store.LoadConfig(dir)
		if err != nil {
			return storeLoadedMsg{err: err}
		}
		st, err := store.LoadState(dir)
		return storeLoadedMsg{config: cfg, state: st, err: err}
	}
}

func (m Model) saveState() (Model, tea.Cmd) {
	if m.saving {
		m.saveQueued = true
		return m, nil
	}
	m.saving = true
	dir, state := m.opts.ConfigDir, store.State{Assignments: m.assignments}
	return m, func() tea.Msg { return stateSavedMsg{err: store.SaveState(dir, state)} }
}

// Init starts the first board load and reads the local files.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchBoard(), m.loadStore(), tea.RequestBackgroundColor)
}

// Update routes messages to the state they change.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
	case tea.BackgroundColorMsg:
		m.theme = newTheme(msg.IsDark())
		m.help.Styles = help.DefaultStyles(msg.IsDark())
	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m.quit()
		}
		if m.picker != nil {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.update(msg)
			return m, cmd
		}
		return m.boardKey(msg)
	case pullRequestsMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.prs = msg.prs
		}
		return m.rebuild(), nil
	case storeLoadedMsg:
		m.storeErr = msg.err
		if msg.err == nil {
			m.storeReady = true
			m.tags = msg.config.Tags
			m.assignments = msg.state.Assignments
		}
		return m.rebuild(), nil
	case moveMsg:
		return m.move(msg.prID, msg.tagID)
	case stateSavedMsg:
		m.saving = false
		m.saveErr = msg.err
		if m.saveQueued {
			m.saveQueued = false
			return m.saveState()
		}
		if m.quitting {
			return m, tea.Quit
		}
	}
	return m, nil
}

// quit waits for an in-flight state write so the last move isn't lost; asking
// a second time quits regardless.
func (m Model) quit() (Model, tea.Cmd) {
	if m.saving && !m.quitting {
		m.quitting = true
		return m, nil
	}
	return m, tea.Quit
}

func (m Model) boardKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.LaneLeft):
		m.focus = max(0, m.focus-1)
	case key.Matches(msg, m.keys.LaneRight):
		m.focus = min(len(m.lanes)-1, m.focus+1)
	case key.Matches(msg, m.keys.CardUp):
		return m.withCursor(m.focus, m.cursors[m.focus]-1), nil
	case key.Matches(msg, m.keys.CardDown):
		return m.withCursor(m.focus, m.cursors[m.focus]+1), nil
	case key.Matches(msg, m.keys.MoveLeft):
		return m.moveSelectedBy(-1)
	case key.Matches(msg, m.keys.MoveRight):
		return m.moveSelectedBy(1)
	case key.Matches(msg, m.keys.MoveTo):
		return m.openMovePicker(), nil
	}
	return m, nil
}

func (m Model) selected() (domain.PullRequest, bool) {
	prs := m.lanes[m.focus].PullRequests
	if len(prs) == 0 {
		return domain.PullRequest{}, false
	}
	return prs[m.cursors[m.focus]], true
}

func (m Model) moveSelectedBy(delta int) (Model, tea.Cmd) {
	pr, ok := m.selected()
	target := m.focus + delta
	if !ok || target < 0 || target >= len(m.lanes) {
		return m, nil
	}
	return m.move(pr.ID, m.lanes[target].Tag.ID)
}

func (m Model) openMovePicker() Model {
	pr, ok := m.selected()
	if !ok || !m.storeReady {
		return m
	}
	items := make([]pickerItem, len(m.lanes))
	for i, lane := range m.lanes {
		items[i] = pickerItem{label: lane.Tag.Name}
		if i == m.focus {
			items[i].note = "(current)"
		}
	}
	lanes := m.lanes
	m.picker = newPicker("Move to…", items, m.focus, func(i int) tea.Msg {
		return moveMsg{prID: pr.ID, tagID: lanes[i].Tag.ID}
	})
	return m
}

// move puts the pull request in the tag's lane, keeps it selected there and
// saves the assignment.
func (m Model) move(prID, tagID string) (Model, tea.Cmd) {
	if !m.storeReady || m.assignedTag(prID) == tagID {
		return m, nil
	}
	m.assignments = board.Assign(m.assignments, prID, tagID)
	m = m.rebuild()
	for li, lane := range m.lanes {
		for ci, pr := range lane.PullRequests {
			if pr.ID == prID {
				m.focus = li
				m = m.withCursor(li, ci)
			}
		}
	}
	return m.saveState()
}

// assignedTag is the ID of the tag whose lane the pull request is shown in,
// empty for Untagged.
func (m Model) assignedTag(prID string) string {
	for _, lane := range m.lanes {
		for _, pr := range lane.PullRequests {
			if pr.ID == prID {
				return lane.Tag.ID
			}
		}
	}
	return ""
}

// withCursor selects card index in lane, kept inside the lane.
func (m Model) withCursor(lane, index int) Model {
	cursors := slices.Clone(m.cursors)
	cursors[lane] = max(0, min(index, len(m.lanes[lane].PullRequests)-1))
	m.cursors = cursors
	return m
}

// rebuild reassembles the lanes and keeps the focus and every lane's cursor
// inside them.
func (m Model) rebuild() Model {
	m.lanes = board.Assemble(m.prs, m.tags, m.assignments)
	cursors := make([]int, len(m.lanes))
	for i, lane := range m.lanes {
		if i < len(m.cursors) {
			cursors[i] = max(0, min(m.cursors[i], len(lane.PullRequests)-1))
		}
	}
	m.cursors = cursors
	m.focus = max(0, min(m.focus, len(m.lanes)-1))
	return m
}
