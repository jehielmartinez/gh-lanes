package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/board"
	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// toastDuration is how long an action's result stays in the status bar.
const toastDuration = 5 * time.Second

// action is something lanes can change about a pull request on GitHub: what
// the status bar says while it runs, once it has worked and when it hasn't,
// and the call that makes it.
type action struct {
	running, done, failed string
	run                   func(gh GitHub, ctx context.Context, id string) error
}

var (
	actionUpdateBranch = action{
		running: "updating branch", done: "branch updated", failed: "couldn't update branch",
		run: func(gh GitHub, ctx context.Context, id string) error {
			return gh.UpdateBranch(ctx, id, domain.UpdateMerge)
		},
	}
	actionRebaseBranch = action{
		running: "rebasing branch", done: "branch rebased", failed: "couldn't rebase branch",
		run: func(gh GitHub, ctx context.Context, id string) error {
			return gh.UpdateBranch(ctx, id, domain.UpdateRebase)
		},
	}
	actionMarkReady = action{
		running: "marking ready for review", done: "marked ready for review", failed: "couldn't mark ready for review",
		run: GitHub.MarkReadyForReview,
	}
	actionConvertToDraft = action{
		running: "converting to draft", done: "converted to draft", failed: "couldn't convert to draft",
		run: GitHub.ConvertToDraft,
	}
)

// toast is a short message in the status bar about the last action.
type toast struct {
	text string
	err  bool
	// until is when it disappears; zero while the action is still running.
	until time.Time
}

// confirmation is an open dialog asking before an action runs.
type confirmation struct {
	title, body string
	act         action
	pr          domain.PullRequest
}

// actionDoneMsg is GitHub's answer to an action.
type actionDoneMsg struct {
	act action
	pr  domain.PullRequest
	err error
}

// prRefreshedMsg is a single pull request fetched again after an action.
type prRefreshedMsg struct {
	id        string
	prs       []domain.PullRequest
	rateLimit domain.RateLimit
	err       error
}

// actionTarget is the pull request actions apply to: the one in the modal,
// or else the selected card.
func (m Model) actionTarget() (domain.PullRequest, bool) {
	if m.detail != nil {
		return m.detail.pr, true
	}
	return m.selected()
}

// actionKeys is the keymap with each action enabled only when it can be taken
// on the target now, so the help footer shows only what will work and a key
// for anything else does nothing.
func (m Model) actionKeys() keyMap {
	k := m.keys
	pr, ok := m.actionTarget()
	ok = ok && !m.acting
	k.UpdateBranch.SetEnabled(ok && pr.CanUpdateBranch())
	k.RebaseBranch.SetEnabled(ok && pr.CanUpdateBranch())
	k.Draft.SetEnabled(ok && (pr.CanMarkReady() || pr.CanConvertToDraft()))
	if pr.IsDraft {
		k.Draft.SetHelp("d", "ready for review")
	} else {
		k.Draft.SetHelp("d", "convert to draft")
	}
	return k
}

// actionKey runs or asks about the action a key press names, reporting false
// when the key names no action that is available.
func (m Model) actionKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	keys := m.actionKeys()
	pr, _ := m.actionTarget()
	switch {
	case key.Matches(msg, keys.UpdateBranch):
		next, cmd := m.act(actionUpdateBranch, pr)
		return next, cmd, true
	case key.Matches(msg, keys.RebaseBranch):
		m.confirm = &confirmation{
			title: "Rebase branch?",
			body:  fmt.Sprintf("Rebase %s onto %s in %s? This rewrites the branch's history.", pr.HeadRef, pr.BaseRef, ref(pr)),
			act:   actionRebaseBranch,
			pr:    pr,
		}
		return m, nil, true
	case key.Matches(msg, keys.Draft) && pr.IsDraft:
		next, cmd := m.act(actionMarkReady, pr)
		return next, cmd, true
	case key.Matches(msg, keys.Draft):
		m.confirm = &confirmation{
			title: "Convert to draft?",
			body:  fmt.Sprintf("Convert %s to a draft? It may dismiss its review requests.", ref(pr)),
			act:   actionConvertToDraft,
			pr:    pr,
		}
		return m, nil, true
	}
	return m, nil, false
}

// confirmKey answers the open confirmation dialog.
func (m Model) confirmKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	keys := newConfirmKeys()
	switch {
	case key.Matches(msg, keys.Confirm):
		c := *m.confirm
		m.confirm = nil
		return m.act(c.act, c.pr)
	case key.Matches(msg, keys.Cancel):
		m.confirm = nil
	}
	return m, nil
}

// act sends the action to GitHub off the UI thread.
func (m Model) act(a action, pr domain.PullRequest) (Model, tea.Cmd) {
	if m.acting {
		return m, nil
	}
	m.acting = true
	m.toast = toast{text: ref(pr) + ": " + a.running + "…"}
	gh := m.opts.GitHub
	run := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		return actionDoneMsg{act: a, pr: pr, err: a.run(gh, ctx, pr.ID)}
	}
	return m, tea.Batch(run, m.spinner.Tick)
}

// actionDone shows how the action went and fetches the pull request again,
// whichever way it went, so the board shows GitHub's state of it.
func (m Model) actionDone(msg actionDoneMsg) (Model, tea.Cmd) {
	m.acting = false
	m.toast = toast{text: ref(msg.pr) + ": " + msg.act.done, until: m.now.Add(toastDuration)}
	if msg.err != nil {
		m.toast = toast{text: ref(msg.pr) + ": " + msg.act.failed + ": " + oneLine(msg.err.Error()), err: true, until: m.now.Add(toastDuration)}
	}
	cmds := []tea.Cmd{m.fetchPullRequest(msg.pr.ID)}
	if m.detail != nil && m.detail.pr.ID == msg.pr.ID {
		var detail tea.Cmd
		m, detail = m.refetchDetail()
		cmds = append(cmds, detail)
	}
	return m, tea.Batch(cmds...)
}

// fetchPullRequest refreshes one pull request on the board.
func (m Model) fetchPullRequest(id string) tea.Cmd {
	gh := m.opts.GitHub
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		prs, limit, err := gh.PullRequestsByID(ctx, []string{id})
		return prRefreshedMsg{id: id, prs: prs, rateLimit: limit, err: err}
	}
}

// prRefreshed puts the fetched pull request in place of the board's copy,
// keeping it selected if it was.
func (m Model) prRefreshed(msg prRefreshedMsg) Model {
	if msg.rateLimit != (domain.RateLimit{}) {
		m.rateLimit = msg.rateLimit
	}
	if msg.err != nil {
		m.toast = toast{text: "Couldn't refresh the pull request: " + oneLine(msg.err.Error()), err: true, until: m.now.Add(toastDuration)}
		return m
	}
	if len(msg.prs) == 0 {
		return m
	}
	selected, wasSelected := m.selected()
	wasSelected = wasSelected && selected.ID == msg.id
	prs := make([]domain.PullRequest, len(m.prs))
	copy(prs, m.prs)
	for i, pr := range prs {
		if pr.ID == msg.id {
			prs[i] = msg.prs[0]
		}
	}
	m.prs = prs
	m = m.rebuild()
	if lane, card, ok := board.Locate(m.lanes, msg.id); ok && wasSelected {
		m.focus = lane
		m = m.withCursor(lane, card)
	}
	return m
}

// toastView is the toast for the status bar, or "" when there is none to
// show.
func (m Model) toastView() string {
	t := m.toast
	switch {
	case t.text == "":
		return ""
	case t.until.IsZero():
		return m.spinner.View() + " " + lipgloss.NewStyle().Foreground(m.theme.text).Render(t.text)
	case !m.now.Before(t.until):
		return ""
	case t.err:
		return lipgloss.NewStyle().Foreground(m.theme.errText).Render(t.text)
	}
	return lipgloss.NewStyle().Foreground(m.theme.success).Render(t.text)
}

func (m Model) confirmView() string {
	c := m.confirm
	width := min(60, max(m.width-4, 20))
	rows := []string{
		lipgloss.NewStyle().Bold(true).Foreground(m.theme.text).Render(c.title),
		"",
		lipgloss.NewStyle().Width(width).Foreground(m.theme.text).Render(oneLine(c.body)),
		"",
		m.help.ShortHelpView(newConfirmKeys().ShortHelp()),
	}
	return lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.accent).
		Render(strings.Join(rows, "\n"))
}

// ref names a pull request the way the board does, like
// "octo-org/sample-repo#12".
func ref(pr domain.PullRequest) string {
	return fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
}
