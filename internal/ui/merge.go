package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

// mergeVerbs are GitHub's own names for merging with each method.
var mergeVerbs = map[domain.MergeMethod]string{
	domain.MergeMethodMerge:  "Create a merge commit",
	domain.MergeMethodSquash: "Squash and merge",
	domain.MergeMethodRebase: "Rebase and merge",
}

// autoMergeNames name each method as auto-merge will use it, as in
// "Squash when ready".
var autoMergeNames = map[domain.MergeMethod]string{
	domain.MergeMethodMerge:  "Merge commit",
	domain.MergeMethodSquash: "Squash",
	domain.MergeMethodRebase: "Rebase",
}

func autoMergeName(method domain.MergeMethod) string {
	if name, ok := autoMergeNames[method]; ok {
		return name
	}
	return "Merge"
}

// mergeDialog is the open merge dialog: the choices GitHub allows the pull
// request now, and whether to delete its branch after a merge.
type mergeDialog struct {
	pr           domain.PullRequest
	offer        domain.MergeOffer
	cursor       int
	deleteBranch bool
}

func newMergeDialog(pr domain.PullRequest) *mergeDialog {
	return &mergeDialog{pr: pr, offer: pr.MergeOffer(), deleteBranch: pr.Repository.DeleteBranchOnMerge}
}

// labels are the dialog's choices, in the order of the repo's merge methods,
// or the one choice of turning auto-merge off.
func (d mergeDialog) labels() []string {
	if d.offer == domain.MergeOfferDisableAutoMerge {
		return []string{"Disable auto-merge"}
	}
	labels := make([]string, len(d.pr.Repository.MergeMethods))
	for i, method := range d.pr.Repository.MergeMethods {
		labels[i] = mergeVerbs[method]
		if d.offer == domain.MergeOfferAutoMerge {
			labels[i] = autoMergeName(method) + " when ready"
		}
	}
	return labels
}

func (d mergeDialog) keys() mergeDialogKeys {
	k := newMergeDialogKeys()
	k.DeleteBranch.SetEnabled(d.offer == domain.MergeOfferMerge)
	return k
}

// mergeDialogKey handles a key press in the merge dialog. Choosing a merge
// asks for confirmation first; auto-merge changes take effect at once.
func (m Model) mergeDialogKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	d := *m.mergeDialog
	keys := d.keys()
	switch {
	case key.Matches(msg, keys.Up):
		d.cursor = max(0, d.cursor-1)
	case key.Matches(msg, keys.Down):
		d.cursor = min(len(d.labels())-1, d.cursor+1)
	case key.Matches(msg, keys.DeleteBranch):
		d.deleteBranch = !d.deleteBranch
	case key.Matches(msg, keys.Close):
		m.mergeDialog = nil
		return m, nil
	case key.Matches(msg, keys.Choose):
		m.mergeDialog = nil
		return m.chooseMerge(d)
	}
	m.mergeDialog = &d
	return m, nil
}

func (m Model) chooseMerge(d mergeDialog) (Model, tea.Cmd) {
	pr := d.pr
	if d.offer == domain.MergeOfferDisableAutoMerge {
		return m.act(actionDisableAutoMerge, pr)
	}
	method := pr.Repository.MergeMethods[d.cursor]
	if d.offer == domain.MergeOfferAutoMerge {
		return m.act(enableAutoMergeAction(method), pr)
	}
	body := fmt.Sprintf("Merge %s into %s (%s)", ref(pr), pr.BaseRef, strings.ToLower(mergeVerbs[method]))
	refID := ""
	if pr.DeletesBranchAfterMerge(d.deleteBranch) {
		refID = pr.HeadRefID
	}
	if d.deleteBranch {
		body += ", then delete " + pr.HeadRef
	}
	m.confirm = &confirmation{
		title: "Merge pull request?",
		body:  body + "?",
		act:   mergeAction(method, pr.HeadRef, refID),
		pr:    pr,
	}
	return m, nil
}

func (m Model) mergeDialogView() string {
	d := *m.mergeDialog
	pr := d.pr
	title, line := "Merge "+ref(pr), pr.MergeSentence()
	switch d.offer {
	case domain.MergeOfferAutoMerge:
		title, line = "Auto-merge "+ref(pr), "Merges by itself once checks pass and required reviews are in."
	case domain.MergeOfferDisableAutoMerge:
		title, line = "Auto-merge "+ref(pr), m.autoMergeLine(pr.AutoMerge)
	}
	text := lipgloss.NewStyle().Foreground(m.theme.text)
	rows := []string{
		lipgloss.NewStyle().Bold(true).Foreground(m.theme.text).Render(title),
		lipgloss.NewStyle().Foreground(m.theme.muted).Render(oneLine(line)),
		"",
	}
	for i, label := range d.labels() {
		marker, style := "  ", text
		if i == d.cursor {
			marker, style = "› ", text.Bold(true).Foreground(m.theme.accent)
		}
		rows = append(rows, marker+style.Render(label))
	}
	if d.offer == domain.MergeOfferMerge {
		box := "[ ]"
		if d.deleteBranch {
			box = "[x]"
		}
		rows = append(rows, "", text.Render(box+" Delete branch "+pr.HeadRef))
	}
	rows = append(rows, "", m.help.ShortHelpView(d.keys().ShortHelp()))
	return modal(m.theme, rows)
}

// mergeDialogKeys are the bindings inside the merge dialog.
type mergeDialogKeys struct {
	Up           key.Binding
	Down         key.Binding
	Choose       key.Binding
	DeleteBranch key.Binding
	Close        key.Binding
}

func newMergeDialogKeys() mergeDialogKeys {
	return mergeDialogKeys{
		Up:           key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:         key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Choose:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "choose")),
		DeleteBranch: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "delete branch")),
		Close:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

func (k mergeDialogKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Choose, k.DeleteBranch, k.Close}
}

// mergeAction merges with the method given and then, when refID is set,
// deletes the head branch it names.
func mergeAction(method domain.MergeMethod, branch, refID string) action {
	return action{
		running: "merging", done: "merged", failed: "couldn't merge",
		run: func(gh GitHub, ctx context.Context, id string) error {
			if err := gh.Merge(ctx, id, method); err != nil {
				return err
			}
			if refID == "" {
				return nil
			}
			if err := gh.DeleteBranch(ctx, refID); err != nil {
				return followUpError{what: "couldn't delete " + branch, err: err}
			}
			return nil
		},
	}
}

func enableAutoMergeAction(method domain.MergeMethod) action {
	return action{
		running: "enabling auto-merge", done: "auto-merge enabled", failed: "couldn't enable auto-merge",
		run: func(gh GitHub, ctx context.Context, id string) error {
			return gh.EnableAutoMerge(ctx, id, method)
		},
	}
}

var actionDisableAutoMerge = action{
	running: "disabling auto-merge", done: "auto-merge disabled", failed: "couldn't disable auto-merge",
	run: GitHub.DisableAutoMerge,
}

// followUpError is an action that went through on GitHub but whose follow-up
// step did not, such as deleting a branch after its merge.
type followUpError struct {
	what string
	err  error
}

func (e followUpError) Error() string { return e.what + ": " + e.err.Error() }

func (e followUpError) Unwrap() error { return e.err }
