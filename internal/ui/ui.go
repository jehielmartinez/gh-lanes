// Package ui is the Bubble Tea front end: the root model, its views and the
// commands that reach the GitHub layer.
package ui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
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
	lanes   []board.Lane
	err     error
}

// New returns the root model, ready to load the board when started.
func New(opts Options) Model {
	return Model{
		opts:    opts,
		keys:    newKeyMap(),
		help:    help.New(),
		theme:   newTheme(true),
		loading: true,
		lanes:   board.Assemble(nil),
	}
}

type pullRequestsMsg struct {
	prs []domain.PullRequest
	err error
}

func (m Model) fetchBoard() tea.Cmd {
	gh := m.opts.GitHub
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, err := gh.SearchPullRequests(ctx, boardSearch)
		return pullRequestsMsg{prs: prs, err: err}
	}
}

// Init starts the first board load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchBoard(), tea.RequestBackgroundColor)
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
			return m, tea.Quit
		}
	case pullRequestsMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.lanes = board.Assemble(msg.prs)
		}
	}
	return m, nil
}
