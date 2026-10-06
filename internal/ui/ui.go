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
	PullRequestsByID(ctx context.Context, ids []string) ([]domain.PullRequest, domain.RateLimit, error)
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
	// prs are the open pull requests and the tagged ones that have since
	// merged or closed.
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
	archived    []domain.Archived

	// One state write is in flight at a time; a move made meanwhile queues
	// another, so writes land in the order the moves were made.
	saving     bool
	saveQueued bool
	saveErr    error
	// Config writes are serialised the same way; each one writes the tags as
	// they stand when it starts.
	savingConfig bool
	configQueued bool
	tagSaveErr   error
	quitting     bool

	lanes      []board.Lane
	focus      int
	cursors    []int
	picker     *picker
	tagManager *tagManager

	// firstLane is the leftmost lane in view, and offsets the first card in
	// view in each lane.
	firstLane int
	offsets   []int

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

// searchedMsg is the open search's result, the first half of a refresh.
type searchedMsg struct {
	open      []domain.PullRequest
	rateLimit domain.RateLimit
	err       error
	at        time.Time
}

// refreshedMsg is a finished refresh: the open pull requests, and the tagged
// ones the search no longer returns, fetched by node ID.
type refreshedMsg struct {
	open, tracked []domain.PullRequest
	rateLimit     domain.RateLimit
	err           error
	at            time.Time
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

// configSavedMsg is the result of writing the tags: the config as read back
// after the write, or why the write or the read failed.
type configSavedMsg struct {
	config  store.Config
	err     error
	loadErr error
}

// moveMsg asks for a pull request to be put in the lane of a tag; an empty
// tag ID is Untagged.
type moveMsg struct{ prID, tagID string }

func (m Model) fetchBoard() tea.Cmd {
	gh, now := m.opts.GitHub, m.opts.Now
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, limit, err := gh.SearchPullRequests(ctx, boardSearch)
		return searchedMsg{open: prs, rateLimit: limit, err: err, at: now()}
	}
}

// fetchTracked completes a refresh by fetching the tagged pull requests the
// open search didn't return.
func (m Model) fetchTracked(searched searchedMsg, ids []string) tea.Cmd {
	gh, now := m.opts.GitHub, m.opts.Now
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, limit, err := gh.PullRequestsByID(ctx, ids)
		if limit == (domain.RateLimit{}) {
			limit = searched.rateLimit
		}
		return refreshedMsg{open: searched.open, tracked: prs, rateLimit: limit, err: err, at: now()}
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
	dir, state := m.opts.ConfigDir, store.State{Assignments: m.assignments, Archived: m.archived}
	return m, func() tea.Msg { return stateSavedMsg{err: store.SaveState(dir, state)} }
}

func (m Model) saveConfig() (Model, tea.Cmd) {
	if m.savingConfig {
		m.configQueued = true
		return m, nil
	}
	m.savingConfig = true
	dir, tags := m.opts.ConfigDir, m.tags
	return m, func() tea.Msg {
		if err := store.SaveTags(dir, tags); err != nil {
			return configSavedMsg{err: err}
		}
		cfg, err := store.LoadConfig(dir)
		return configSavedMsg{config: cfg, loadErr: err}
	}
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

// Update routes messages to the state they change, then scrolls whatever
// that moved back into view.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	return next.scrolled().syncDetail(), cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
	case tea.BackgroundColorMsg:
		m.theme = newTheme(msg.IsDark())
		m.help.Styles = help.DefaultStyles(msg.IsDark())
	case tea.PasteMsg:
		if m.tagManager != nil {
			return m.tagManagerUpdate(msg)
		}
	case tea.KeyPressMsg:
		if m.tagManager != nil && m.tagManager.typing() && !key.Matches(msg, m.keys.Interrupt) {
			return m.tagManagerUpdate(msg)
		}
		if key.Matches(msg, m.keys.Quit) {
			return m.quit()
		}
		if m.picker != nil {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.update(msg)
			return m, cmd
		}
		if m.tagManager != nil {
			return m.tagManagerUpdate(msg)
		}
		if key.Matches(msg, m.keys.Tags) {
			return m.openTagManager(), nil
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
		if m.detail == nil && m.picker == nil && m.tagManager == nil {
			return m.clickBoard(msg)
		}
	case tea.MouseWheelMsg:
		if m.detail != nil {
			return m.detailScroll(msg), nil
		}
		if m.picker == nil && m.tagManager == nil {
			return m.wheeled(msg), nil
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
	case searchedMsg:
		if msg.err == nil {
			if missing := board.Missing(msg.open, board.Tagged(m.assignments, m.tags)); len(missing) > 0 {
				return m, m.fetchTracked(msg, missing)
			}
		}
		return m.refreshed(refreshedMsg{open: msg.open, rateLimit: msg.rateLimit, err: msg.err, at: msg.at})
	case refreshedMsg:
		return m.refreshed(msg)
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
			m.archived = msg.state.Archived
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
		if m.quitting && !m.savingConfig {
			return m, tea.Quit
		}
	case configSavedMsg:
		m.savingConfig = false
		if m.configQueued {
			m.configQueued = false
			return m.saveConfig()
		}
		m = m.configSaved(msg)
		if m.quitting && !m.saving {
			return m, tea.Quit
		}
	}
	return m, nil
}

// configSaved takes up the config as it was read back after a write, which
// also picks up any setting edited by hand since the app started.
func (m Model) configSaved(msg configSavedMsg) Model {
	m.tagSaveErr = msg.err
	if msg.err != nil {
		return m
	}
	var intervalErr *store.RefreshIntervalError
	if msg.loadErr != nil && !errors.As(msg.loadErr, &intervalErr) {
		m.tagSaveErr = msg.loadErr
		return m
	}
	m.configErr = msg.loadErr
	m.tags = msg.config.Tags
	m.refreshInterval = msg.config.RefreshInterval
	return m.rebuild()
}

// openTagManager opens the tag manager on the focused lane's tag. It stays
// shut while the config couldn't be read, since saving would overwrite it.
func (m Model) openTagManager() Model {
	if m.storeReady {
		m.tagManager = newTagManager(m.focus)
	}
	return m
}

func (m Model) tagManagerUpdate(msg tea.Msg) (Model, tea.Cmd) {
	var edit *tagEdit
	m.tagManager, edit = m.tagManager.update(msg, m.tags, m.assignments)
	if edit == nil {
		return m, nil
	}
	m.tags = edit.tags
	var saveState tea.Cmd
	if edit.assignments != nil {
		m.assignments = edit.assignments
		m, saveState = m.saveState()
	}
	m = m.rebuild()
	m, saveConfig := m.saveConfig()
	return m, tea.Batch(saveState, saveConfig)
}

// refreshed takes in a finished refresh. A failed one keeps the board it had;
// a successful one replaces it and brings the archived list up to date,
// saving it when that changed.
func (m Model) refreshed(msg refreshedMsg) (Model, tea.Cmd) {
	m.refreshing = false
	m.now, m.lastRefresh = msg.at, msg.at
	m.err = msg.err
	if msg.err != nil {
		return m.rebuild(), nil
	}
	m.loaded = true
	m.updatedAt = msg.at
	m.prs = board.Retain(msg.open, msg.tracked, board.Tagged(m.assignments, m.tags))
	m.rateLimit = msg.rateLimit
	archived, changed := board.Reconcile(m.archived, msg.open)
	if !changed || !m.storeReady {
		return m.rebuild(), nil
	}
	m.archived = archived
	return m.rebuild().saveState()
}

// quit waits for in-flight writes so the last change isn't lost; asking a
// second time quits regardless.
func (m Model) quit() (Model, tea.Cmd) {
	if (m.saving || m.savingConfig) && !m.quitting {
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
	case key.Matches(msg, m.keys.Archive):
		return m.archiveSelected()
	case key.Matches(msg, m.keys.Open):
		if pr, ok := m.selected(); ok {
			return m.openDetail(pr)
		}
	case key.Matches(msg, m.keys.Refresh):
		return m.refresh()
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.keys = m.keys.withFullHelp(m.help.ShowAll)
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

// archiveSelected takes the selected card off the board until its pull
// request is reopened, and saves that.
func (m Model) archiveSelected() (Model, tea.Cmd) {
	pr, ok := m.selected()
	if !ok || !m.storeReady {
		return m, nil
	}
	m.assignments, m.archived = board.Archive(m.assignments, m.archived, pr)
	return m.rebuild().saveState()
}

// withCursor selects card index in lane, kept inside the lane.
func (m Model) withCursor(lane, index int) Model {
	cursors := slices.Clone(m.cursors)
	cursors[lane] = max(0, min(index, len(m.lanes[lane].PullRequests)-1))
	m.cursors = cursors
	return m
}

// rebuild reassembles the lanes. The focus and each lane's cursor and scroll
// offset stay with their tag when tags are reordered, and the cursors are kept
// inside the lanes.
func (m Model) rebuild() Model {
	cursorOf, offsetOf := map[string]int{}, map[string]int{}
	for i, lane := range m.lanes {
		if i < len(m.cursors) {
			cursorOf[lane.Tag.ID] = m.cursors[i]
			offsetOf[lane.Tag.ID] = m.offsets[i]
		}
	}
	focused, hadFocus := "", m.focus < len(m.lanes)
	if hadFocus {
		focused = m.lanes[m.focus].Tag.ID
	}

	m.lanes = board.Assemble(m.prs, m.tags, m.assignments, m.archived)
	cursors := make([]int, len(m.lanes))
	offsets := make([]int, len(m.lanes))
	for i, lane := range m.lanes {
		cursors[i] = max(0, min(cursorOf[lane.Tag.ID], len(lane.PullRequests)-1))
		offsets[i] = offsetOf[lane.Tag.ID]
		if hadFocus && lane.Tag.ID == focused {
			m.focus = i
		}
	}
	m.cursors, m.offsets = cursors, offsets
	m.focus = max(0, min(m.focus, len(m.lanes)-1))
	return m
}

// clickBoard selects the card under a left click, and opens it when the
// click follows another on the same card quickly enough to make a double
// click.
func (m Model) clickBoard(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	lane, card, ok := m.cardAt(msg.X, msg.Y)
	if msg.Button != tea.MouseLeft || !ok {
		m.lastClick = click{}
		return m, nil
	}
	m = m.clicked(msg)
	pr := m.lanes[lane].PullRequests[card]
	now := m.opts.Now()
	if m.lastClick.prID == pr.ID && now.Sub(m.lastClick.at) <= doubleClick {
		m.lastClick = click{}
		return m.openDetail(pr)
	}
	m.lastClick = click{prID: pr.ID, at: now}
	return m, nil
}
