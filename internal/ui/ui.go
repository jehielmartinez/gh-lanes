// Package ui is the Bubble Tea front end: the root model, its views and the
// commands that reach the GitHub layer and the local files.
package ui

import (
	"context"
	"errors"
	"slices"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
	"github.com/jehielmartinez/gh-lanes/internal/store"
)

const boardSearch = "is:pr is:open author:@me archived:false"

const fetchTimeout = 30 * time.Second

// doubleClick is the longest gap between two clicks on a card that still
// counts as a double click.
const doubleClick = 500 * time.Millisecond

// clockTick is how often the clock is sampled, which is what keeps
// "updated Ns ago" and card ages current and fires refreshes when due.
const clockTick = time.Second

// GitHub is what the UI needs from the GitHub layer.
type GitHub interface {
	SearchPullRequests(ctx context.Context, query string) ([]domain.PullRequest, domain.RateLimit, error)
	PullRequest(ctx context.Context, id string) (domain.PullRequest, domain.RateLimit, error)
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
	// After waits for a duration on the same clock as Now, like time.After.
	After func(time.Duration) <-chan time.Time
}

// Model is the root model of the app.
type Model struct {
	opts    Options
	keys    keyMap
	help    help.Model
	spinner spinner.Model
	theme   theme

	width, height int

	now             time.Time
	refreshInterval time.Duration
	configErr       error

	refreshing bool
	// lastRefresh is when the last refresh finished, successful or not; the
	// next automatic one is due an interval after it.
	lastRefresh time.Time
	// loaded is whether the board holds data from a successful refresh.
	loaded    bool
	updatedAt time.Time
	prs       []domain.PullRequest
	rateLimit domain.RateLimit
	// err is the last refresh's error. With loaded set, the board on screen
	// is stale.
	err error

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

	// detail is the open detail modal, nil while the board has the keys.
	detail    *detail
	detailSeq int
	lastClick click
}

// click is a left click that landed on a card.
type click struct {
	prID string
	at   time.Time
}

// New returns the root model, ready to load the board when started.
func New(opts Options) Model {
	m := Model{
		opts:            opts,
		keys:            newKeyMap(),
		help:            help.New(),
		spinner:         spinner.New(spinner.WithSpinner(spinner.Dot)),
		theme:           newTheme(true),
		now:             opts.Now(),
		refreshInterval: store.DefaultRefreshInterval,
		refreshing:      true,
	}
	return m.rebuild()
}

type pullRequestsMsg struct {
	prs       []domain.PullRequest
	rateLimit domain.RateLimit
	err       error
	at        time.Time
}

type tickMsg struct {
	now time.Time
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
	gh, now := m.opts.GitHub, m.opts.Now
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, limit, err := gh.SearchPullRequests(ctx, boardSearch)
		return pullRequestsMsg{prs: prs, rateLimit: limit, err: err, at: now()}
	}
}

func (m Model) tick() tea.Cmd {
	after := m.opts.After
	return func() tea.Msg {
		return tickMsg{now: <-after(clockTick)}
	}
}

// refresh starts a refresh unless one is already in flight.
func (m Model) refresh() (Model, tea.Cmd) {
	if m.refreshing {
		return m, nil
	}
	m.refreshing = true
	return m, tea.Batch(m.fetchBoard(), m.spinner.Tick)
}

func (m Model) loadStore() tea.Cmd {
	dir := m.opts.ConfigDir
	return func() tea.Msg {
		cfg, err := store.LoadConfig(dir)
		var intervalErr *store.RefreshIntervalError
		if err != nil && !errors.As(err, &intervalErr) {
			return storeLoadedMsg{config: cfg, err: err}
		}
		st, stateErr := store.LoadState(dir)
		if stateErr != nil {
			return storeLoadedMsg{config: cfg, err: stateErr}
		}
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

// Init reads the local files, then starts the first board load and the
// clock. The config comes first so no tick can be measured against the
// default interval once the file has set another.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Sequence(m.loadStore(), tea.Batch(m.fetchBoard(), m.tick())),
		m.spinner.Tick,
		tea.RequestBackgroundColor,
	)
}

// Update routes messages to the state they change.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	return next.syncDetail(), cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
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
		if key.Matches(msg, m.keys.Refresh) {
			m, board := m.refresh()
			m, detail := m.fetchDetail()
			return m, tea.Batch(board, detail)
		}
		if m.detail != nil {
			return m.detailKey(msg)
		}
		return m.boardKey(msg)
	case tea.MouseClickMsg:
		if m.detail == nil && m.picker == nil && msg.Button == tea.MouseLeft {
			return m.clickBoard(msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		if m.detail != nil {
			return m.detailScroll(msg), nil
		}
	case tickMsg:
		m.now = msg.now
		next := m.tick()
		m, detail := m.detailDue()
		if m.now.Sub(m.lastRefresh) < m.refreshInterval {
			return m, tea.Batch(next, detail)
		}
		m, board := m.refresh()
		return m, tea.Batch(next, detail, board)
	case spinner.TickMsg:
		if !m.refreshing && (m.detail == nil || !m.detail.fetching) {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case pullRequestsMsg:
		m.refreshing = false
		m.now, m.lastRefresh = msg.at, msg.at
		m.err = msg.err
		if msg.err == nil {
			m.loaded = true
			m.updatedAt = msg.at
			m.prs = msg.prs
			m.rateLimit = msg.rateLimit
		}
		return m.rebuild(), nil
	case detailMsg:
		return m.detailFetched(msg), nil
	case storeLoadedMsg:
		m.refreshInterval = msg.config.RefreshInterval
		var intervalErr *store.RefreshIntervalError
		if errors.As(msg.err, &intervalErr) {
			m.configErr = msg.err
		} else {
			m.storeErr = msg.err
		}
		if m.storeErr == nil {
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
	case key.Matches(msg, m.keys.Open):
		if pr, ok := m.selected(); ok {
			return m.openDetail(pr)
		}
	case key.Matches(msg, m.keys.Refresh):
		return m.refresh()
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
	from, _, onBoard := board.Locate(m.lanes, prID)
	if !m.storeReady || !onBoard || m.lanes[from].Tag.ID == tagID {
		return m, nil
	}
	m.assignments = board.Assign(m.assignments, prID, tagID)
	m = m.rebuild()
	if lane, card, ok := board.Locate(m.lanes, prID); ok {
		m.focus = lane
		m = m.withCursor(lane, card)
	}
	return m.saveState()
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

// clickBoard opens the card under a click that follows another on the same
// card quickly enough to make a double click.
func (m Model) clickBoard(x, y int) (Model, tea.Cmd) {
	pr, ok := m.cardAt(x, y)
	if !ok {
		m.lastClick = click{}
		return m, nil
	}
	now := m.opts.Now()
	if m.lastClick.prID == pr.ID && now.Sub(m.lastClick.at) <= doubleClick {
		m.lastClick = click{}
		return m.openDetail(pr)
	}
	m.lastClick = click{prID: pr.ID, at: now}
	return m, nil
}
