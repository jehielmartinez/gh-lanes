// Package ui is the Bubble Tea front end: the root model, its views and the
// commands that reach the GitHub layer.
package ui

import (
	"context"
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

// clockTick is how often the clock is sampled, which is what keeps
// "updated Ns ago" and card ages current and fires refreshes when due.
const clockTick = time.Second

// GitHub is what the UI needs from the GitHub layer.
type GitHub interface {
	SearchPullRequests(ctx context.Context, query string) ([]domain.PullRequest, domain.RateLimit, error)
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
	lanes     []board.Lane
	rateLimit domain.RateLimit
	// err is the last refresh's error. With loaded set, the board on screen
	// is stale.
	err error
}

// New returns the root model, ready to load the board when started.
func New(opts Options) Model {
	return Model{
		opts:            opts,
		keys:            newKeyMap(),
		help:            help.New(),
		spinner:         spinner.New(spinner.WithSpinner(spinner.Dot)),
		theme:           newTheme(true),
		now:             opts.Now(),
		refreshInterval: store.DefaultRefreshInterval,
		refreshing:      true,
		lanes:           board.Assemble(nil),
	}
}

type configMsg struct {
	cfg store.Config
	err error
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

func (m Model) loadConfig() tea.Cmd {
	dir := m.opts.ConfigDir
	return func() tea.Msg {
		cfg, err := store.LoadConfig(dir)
		return configMsg{cfg: cfg, err: err}
	}
}

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

// Init loads the config, then starts the first board load and the clock. The
// config comes first so no tick can be measured against the default interval
// once the file has set another.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Sequence(m.loadConfig(), tea.Batch(m.fetchBoard(), m.tick())),
		m.spinner.Tick,
		tea.RequestBackgroundColor,
	)
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
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			return m.refresh()
		}
	case configMsg:
		m.refreshInterval = msg.cfg.RefreshInterval
		m.configErr = msg.err
	case tickMsg:
		m.now = msg.now
		next := m.tick()
		if m.now.Sub(m.lastRefresh) < m.refreshInterval {
			return m, next
		}
		m, cmd := m.refresh()
		return m, tea.Batch(next, cmd)
	case spinner.TickMsg:
		if !m.refreshing {
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
			m.lanes = board.Assemble(msg.prs)
			m.rateLimit = msg.rateLimit
		}
	}
	return m, nil
}
