package ui_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const managerTitle = "Tags"

// openTagManager opens the tag manager over the board.
func (h *harness) openTagManager() string {
	h.t.Helper()
	h.press("t")
	return h.waitForText("esc close")
}

// waitForConfig waits until the config file satisfies cond and returns it.
func (h *harness) waitForConfig(desc string, cond func(configYAML) bool) configYAML {
	h.t.Helper()
	var cfg configYAML
	h.waitFor("config "+desc, func() bool {
		raw, err := os.ReadFile(filepath.Join(h.configDir, "config.yaml"))
		if err != nil {
			return false
		}
		cfg = configYAML{}
		return yaml.Unmarshal(raw, &cfg) == nil && cond(cfg)
	})
	return cfg
}

func tagNames(cfg configYAML) []string {
	var names []string
	for _, tag := range cfg.Tags {
		names = append(names, tag.Name)
	}
	return names
}

func tagByName(cfg configYAML, name string) (id, color string, terminal, ok bool) {
	for _, tag := range cfg.Tags {
		if tag.Name == name {
			return tag.ID, tag.Color, tag.Terminal, true
		}
	}
	return "", "", false, false
}

// laneOrder is the lane names in the order the header row shows them.
func laneOrder(screen string, names ...string) []string {
	header := headerLine(screen)
	sorted := slices.Clone(names)
	slices.SortFunc(sorted, func(a, b string) int {
		return strings.Index(header, a+" ") - strings.Index(header, b+" ")
	})
	return sorted
}

var hexColor = regexp.MustCompile(`^#[0-9A-F]{6}$`)

func TestTagManagerOpensOnTheFocusedLane(t *testing.T) {
	h := startBoard(t)
	h.press("l")
	h.press("l")
	h.openTagManager()
	h.press("r")
	h.waitForText("Rename Review")
}

func TestTagManagerListsUntaggedFirstThenEveryTag(t *testing.T) {
	h := startBoard(t)

	screen := h.openTagManager()
	if !strings.Contains(screen, managerTitle) {
		t.Errorf("tag manager title missing:\n%s", screen)
	}
	body := screen[strings.Index(screen, managerTitle):]
	last := -1
	for _, name := range []string{"Untagged", "In Progress", "Review", "Testing", "Demo", "Done"} {
		i := strings.Index(body, name)
		if i <= last {
			t.Fatalf("tag manager should list %q after the previous tag:\n%s", name, screen)
		}
		last = i
	}

	h.press("esc")
	h.waitForScreen("the tag manager to close", func(s string) bool { return !strings.Contains(s, "esc close") })
}

func TestCreatingATagAddsALaneWithAStableID(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()

	h.press("n")
	h.waitForText("New tag")
	h.press("Code Freeze")
	h.press("enter")
	screen := h.waitForText("Color for Code Freeze")
	shown := regexp.MustCompile(`#[0-9A-F]{6}`).FindString(screen[strings.Index(screen, "Color for"):])
	h.press("enter")

	cfg := h.waitForConfig("with the new tag", func(c configYAML) bool {
		_, _, _, ok := tagByName(c, "Code Freeze")
		return ok
	})
	if want := []string{"In Progress", "Review", "Testing", "Demo", "Done", "Code Freeze"}; !slices.Equal(tagNames(cfg), want) {
		t.Errorf("tags = %v, want %v", tagNames(cfg), want)
	}
	id, color, terminal, _ := tagByName(cfg, "Code Freeze")
	if id != "code-freeze" {
		t.Errorf("new tag id = %q, want code-freeze", id)
	}
	if !hexColor.MatchString(color) || color != shown {
		t.Errorf("new tag color = %q, want the color shown when choosing (%q)", color, shown)
	}
	for _, other := range cfg.Tags {
		if other.Name != "Code Freeze" && other.Color == color {
			t.Errorf("a new tag should start on a color no other tag uses; %s already has %s", other.Name, color)
		}
	}
	if terminal {
		t.Errorf("a new tag should not be terminal")
	}
	if cfg.Version != 1 {
		t.Errorf("config version = %d, want 1", cfg.Version)
	}

	h.press("esc")
	h.waitForText("Code Freeze 0")
}

func TestCreatingATagWithANameTakenByADeletedTagDoesNotRevivePRs(t *testing.T) {
	h := startBoard(t)
	h.press("L")
	h.press("L")
	h.waitForAssignments(map[string]string{newestPR: "review"})

	h.openTagManager()
	h.press("d")
	h.press("y")
	h.waitForAssignments(map[string]string{})
	h.press("n")
	h.press("Review")
	h.press("enter")
	h.waitForText("Color for Review")
	h.press("enter")
	h.press("esc")

	h.waitForText("Review 0")
	if !strings.Contains(headerLine(h.screen.plain()), "Untagged 3") {
		t.Errorf("a recreated tag must start empty:\n%s", h.screen.plain())
	}
}

func TestNewTagNamesMustBeUniqueAndNonEmpty(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()

	for _, name := range []string{"   ", "review", "Untagged"} {
		h.press("n")
		h.waitForText("New tag")
		h.press(name)
		h.press("enter")
		h.waitForScreen("the name to be refused", func(s string) bool {
			return strings.Contains(s, "needs a name") || strings.Contains(s, "already exists")
		})
		h.press("esc")
		h.waitForScreen("the form to close", func(s string) bool { return !strings.Contains(s, "New tag") })
	}

	h.press("esc")
	h.press("q")
	h.waitFinished()
	if names := tagNames(readConfig(t, h.configDir)); len(names) != 5 {
		t.Errorf("refused names must not be saved; tags = %v", names)
	}
}

func TestTypingATagNameDoesNotTriggerBoardKeys(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()

	h.press("n")
	h.waitForText("New tag")
	h.press("quit LH rtm")
	h.press("enter")
	h.waitForText("Color for quit LH rtm")
	h.press("enter")
	h.waitForConfig("with the typed tag", func(c configYAML) bool {
		_, _, _, ok := tagByName(c, "quit LH rtm")
		return ok
	})
	if s, _ := readState(h.configDir); len(s.Assignments) > 0 {
		t.Errorf("letters typed into a name must not move cards, got %v", s.Assignments)
	}
}

func TestRenamingATagKeepsItsPRs(t *testing.T) {
	h := startBoard(t)
	h.press("L")
	h.waitForAssignments(map[string]string{newestPR: "in-progress"})

	h.openTagManager()
	h.press("r")
	h.waitForText("Rename In Progress")
	h.press("ctrl+u")
	h.press("Doing")
	h.press("enter")

	cfg := h.waitForConfig("with the renamed tag", func(c configYAML) bool {
		_, _, _, ok := tagByName(c, "Doing")
		return ok
	})
	if id, _, _, _ := tagByName(cfg, "Doing"); id != "in-progress" {
		t.Errorf("renamed tag id = %q, want it kept as in-progress", id)
	}
	h.press("esc")
	h.waitForText("Doing 1")
	h.waitForAssignments(map[string]string{newestPR: "in-progress"})
}

func TestRecoloringATagSavesTheChosenColor(t *testing.T) {
	h := startBoard(t)
	before := readConfig(t, h.configDir)
	_, oldColor, _, _ := tagByName(before, "Review")

	h.openTagManager()
	h.press("j")
	h.press("j")
	h.press("c")
	h.waitForText("Color for Review")
	h.press("l")
	screen := h.waitForScreen("another color to be chosen", func(s string) bool {
		i := strings.Index(s, "Color for Review")
		return i >= 0 && !strings.Contains(s[i:], oldColor)
	})
	shown := regexp.MustCompile(`#[0-9A-F]{6}`).FindString(screen[strings.Index(screen, "Color for Review"):])
	h.press("enter")

	cfg := h.waitForConfig("with Review recolored", func(c configYAML) bool {
		_, color, _, _ := tagByName(c, "Review")
		return color != oldColor
	})
	if _, color, _, _ := tagByName(cfg, "Review"); color != shown {
		t.Errorf("Review color = %q, want %q", color, shown)
	}
}

func TestCancellingAColorChangeKeepsTheOldColor(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()
	h.press("j")
	h.press("c")
	h.waitForText("Color for In Progress")
	h.press("l")
	h.press("esc")
	h.waitForScreen("the color chooser to close", func(s string) bool { return !strings.Contains(s, "Color for") })
	h.press("esc")
	h.press("q")
	h.waitFinished()

	before := `#3B82F6`
	if _, color, _, _ := tagByName(readConfig(t, h.configDir), "In Progress"); color != before {
		t.Errorf("In Progress color = %q, want it unchanged", color)
	}
}

func TestReorderingTagsReordersLanes(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()

	h.press("j")
	h.press("J")
	h.press("J")
	h.waitForConfig("with In Progress moved down two", func(c configYAML) bool {
		return slices.Equal(tagNames(c), []string{"Review", "Testing", "In Progress", "Demo", "Done"})
	})
	h.press("K")
	h.waitForConfig("with In Progress moved back up one", func(c configYAML) bool {
		return slices.Equal(tagNames(c), []string{"Review", "In Progress", "Testing", "Demo", "Done"})
	})

	h.press("esc")
	screen := h.waitForScreen("the manager to close", func(s string) bool { return !strings.Contains(s, "esc close") })
	lanes := []string{"Untagged", "Review", "In Progress", "Testing", "Demo", "Done"}
	if got := laneOrder(screen, lanes...); !slices.Equal(got, lanes) {
		t.Errorf("lanes = %v, want %v:\n%s", got, lanes, headerLine(screen))
	}
}

func TestATagCannotMoveAboveUntaggedOrPastTheEnd(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()

	h.press("j")
	h.press("K")
	for range 6 {
		h.press("j")
	}
	h.press("J")
	h.press("k")
	h.press("J")
	h.waitForConfig("with Demo and Done swapped", func(c configYAML) bool {
		return slices.Equal(tagNames(c), []string{"In Progress", "Review", "Testing", "Done", "Demo"})
	})
}

func TestDeletingATagMovesItsPRsToUntagged(t *testing.T) {
	h := startBoard(t)
	h.press("L")
	h.press("h")
	h.press("L")
	h.press("L")
	h.waitForAssignments(map[string]string{olderPR: "review", newestPR: "in-progress"})

	h.openTagManager()
	h.press("d")
	h.waitForText("Delete Review?")
	h.press("y")

	cfg := h.waitForConfig("without Review", func(c configYAML) bool {
		_, _, _, ok := tagByName(c, "Review")
		return !ok
	})
	if want := []string{"In Progress", "Testing", "Demo", "Done"}; !slices.Equal(tagNames(cfg), want) {
		t.Errorf("tags = %v, want %v", tagNames(cfg), want)
	}
	h.waitForAssignments(map[string]string{newestPR: "in-progress"})
	h.press("esc")
	h.waitForText("Untagged 2")
	if strings.Contains(headerLine(h.screen.plain()), "Review") {
		t.Errorf("the deleted lane is still shown:\n%s", h.screen.plain())
	}
}

func TestDeleteAsksFirstAndCanBeCancelled(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()
	h.press("j")
	h.press("d")
	h.waitForText("Delete In Progress?")
	h.press("n")
	h.waitForScreen("the question to close", func(s string) bool { return !strings.Contains(s, "Delete In Progress?") })
	h.press("d")
	h.waitForText("Delete In Progress?")
	h.press("esc")
	h.waitForScreen("the question to close", func(s string) bool { return !strings.Contains(s, "Delete In Progress?") })
	h.press("esc")
	h.press("q")
	h.waitFinished()

	if _, _, _, ok := tagByName(readConfig(t, h.configDir), "In Progress"); !ok {
		t.Errorf("a cancelled delete must keep the tag")
	}
}

func TestTogglingTheTerminalFlag(t *testing.T) {
	h := startBoard(t)
	h.openTagManager()
	h.press("j")
	h.press("j")
	h.press("t")
	h.waitForConfig("with Review terminal", func(c configYAML) bool {
		_, _, terminal, _ := tagByName(c, "Review")
		return terminal
	})
	h.waitForScreen("Review marked terminal", func(s string) bool {
		return regexp.MustCompile(`Review\s+\d+\s+terminal`).MatchString(s)
	})

	h.press("t")
	h.waitForConfig("with Review no longer terminal", func(c configYAML) bool {
		_, _, terminal, ok := tagByName(c, "Review")
		return ok && !terminal
	})
}

func TestUntaggedCannotBeChanged(t *testing.T) {
	h := startBoard(t)
	before, _ := os.ReadFile(filepath.Join(h.configDir, "config.yaml"))
	h.openTagManager()

	for _, k := range []string{"r", "d", "c", "t", "J"} {
		h.press(k)
		h.waitForText("Untagged can't be changed")
		if s := h.screen.plain(); strings.Contains(s, "Rename") || strings.Contains(s, "Delete Untagged") || strings.Contains(s, "Color for") {
			t.Fatalf("%q on Untagged opened an editor:\n%s", k, s)
		}
	}
	h.press("esc")
	h.press("q")
	h.waitFinished()

	after, _ := os.ReadFile(filepath.Join(h.configDir, "config.yaml"))
	if string(before) != string(after) {
		t.Errorf("config changed:\n%s", after)
	}
	if !strings.HasPrefix(headerLine(h.screen.plain()), "Untagged 3") {
		t.Errorf("Untagged should still be first:\n%s", h.screen.plain())
	}
}

func TestSavingTagsKeepsTheRestOfTheConfig(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", "version: 1\nrefresh_interval: 5m\ntags:\n  - id: tag-b\n    name: Bravo\n    color: \"#FF0000\"\n")
	h := startBoard(t, withConfigDir(dir))

	h.openTagManager()
	h.press("j")
	h.press("t")
	h.waitForConfig("with Bravo terminal", func(c configYAML) bool {
		_, _, terminal, _ := tagByName(c, "Bravo")
		return terminal
	})
	raw, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if !strings.Contains(string(raw), "refresh_interval: 5m") {
		t.Errorf("saving tags dropped the refresh interval:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"#FF0000"`) && !strings.Contains(string(raw), "'#FF0000'") {
		t.Errorf("saving tags changed a hand-set color:\n%s", raw)
	}
	h.press("esc")
	h.press("q")
	h.waitFinished()

	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"config.yaml", "state.json"}; !reflect.DeepEqual(names, want) {
		t.Errorf("config dir holds %v, want %v (no temp files)", names, want)
	}
}

func TestTagChangesSurviveARestart(t *testing.T) {
	first := startBoard(t)
	first.openTagManager()
	first.press("n")
	first.press("Blocked")
	first.press("enter")
	first.waitForText("Color for Blocked")
	first.press("enter")
	first.press("k")
	first.press("d")
	first.press("y")
	first.waitForConfig("with Done gone and Blocked added", func(c configYAML) bool {
		return slices.Equal(tagNames(c), []string{"In Progress", "Review", "Testing", "Demo", "Blocked"})
	})
	first.press("esc")
	first.press("q")
	first.waitFinished()

	h := startBoard(t, withConfigDir(first.configDir))
	lanes := []string{"Untagged", "In Progress", "Review", "Testing", "Demo", "Blocked"}
	screen := h.waitForText("Blocked 0")
	if got := laneOrder(screen, lanes...); !slices.Equal(got, lanes) {
		t.Errorf("lanes after restart = %v, want %v", got, lanes)
	}
	if strings.Contains(headerLine(screen), "Done") {
		t.Errorf("deleted tag came back after a restart:\n%s", headerLine(screen))
	}
}

func TestTagManagerDoesNotOpenWhenTheConfigIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", "tags: [\n")
	h := newHarness(t, boardTransport(t), withConfigDir(dir))
	h.waitForText("Couldn't load tags")

	h.press("t")
	h.press("n")
	h.press("x")
	h.press("enter")
	h.press("enter")
	h.press("q")
	h.waitFinished()

	raw, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if string(raw) != "tags: [\n" {
		t.Errorf("an unreadable config was rewritten:\n%s", raw)
	}
}

func TestFullHelpOffersTheTagManager(t *testing.T) {
	h := startBoard(t)
	h.press("?")
	h.waitForScreen("t in the full help", func(s string) bool {
		return regexp.MustCompile(`\bt\s+tags\b`).MatchString(s)
	})
}

func TestTagManagerOpensOverTheDetailModal(t *testing.T) {
	h := newModalHarness(t, "detail_behind.json")
	h.press("enter")
	h.waitForDetailRequests(1)
	h.waitForText("page down")
	h.press("t")
	h.waitForText("n new")
	h.press("esc")
	h.waitForScreen("the tag manager to close", func(s string) bool { return !strings.Contains(s, "n new") })
}
