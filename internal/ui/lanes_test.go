package ui_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

const (
	newestPR = "PR_node_newer"     // user-a/other-repo#42, first in its lane
	olderPR  = "PR_node_older"     // octo-org/sample-repo#7, second
	oldestPR = "PR_node_long_repo" // octo-org/a-very-long-sample-repository-name#1234, third
)

type configYAML struct {
	Version int `yaml:"version"`
	Tags    []struct {
		ID       string `yaml:"id"`
		Name     string `yaml:"name"`
		Color    string `yaml:"color"`
		Terminal bool   `yaml:"terminal"`
	} `yaml:"tags"`
}

type stateJSON struct {
	Version     int               `json:"version"`
	Assignments map[string]string `json:"assignments"`
	Archived    []archivedJSON    `json:"archived"`
}

type archivedJSON struct {
	ID   string `json:"id"`
	Open bool   `json:"open"`
	Tag  string `json:"tag,omitempty"`
}

const customConfig = `version: 1
tags:
  - id: tag-b
    name: Bravo
    color: "#FF0000"
  - id: tag-a
    name: Alpha
    color: "#00FF00"
    terminal: true
`

func boardTransport(t *testing.T) *githubtest.Transport {
	t.Helper()
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	return transport
}

// startBoard starts the app on the three-PR board and waits until both the
// pull requests and the tag lanes are on screen.
func startBoard(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()
	h := newHarness(t, boardTransport(t), opts...)
	h.waitForScreen("the board with its tag lanes", func(s string) bool {
		return onScreen(s, "octo-org/sample-repo#7") && hasTagLanes(s)
	})
	return h
}

// hasTagLanes reports whether the header row shows more than the Untagged lane.
func hasTagLanes(screen string) bool {
	return len(strings.Fields(headerLine(screen))) > 2
}

func seedFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, dir string) configYAML {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var c configYAML
	if err := yaml.Unmarshal(raw, &c); err != nil {
		t.Fatalf("config is not YAML: %v\n%s", err, raw)
	}
	return c
}

func readState(dir string) (stateJSON, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return stateJSON{}, false
	}
	var s stateJSON
	if err := json.Unmarshal(raw, &s); err != nil {
		return stateJSON{}, false
	}
	return s, true
}

// waitForAssignments waits until the state file holds exactly want.
func (h *harness) waitForAssignments(want map[string]string) {
	h.t.Helper()
	h.waitFor("state file assignments "+formatMap(want), func() bool {
		s, ok := readState(h.configDir)
		return ok && s.Version == 1 && reflect.DeepEqual(s.Assignments, want)
	})
}

func formatMap(m map[string]string) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// headerLine is the row of lane headers, under the tab bar.
func headerLine(screen string) string {
	return strings.Split(screen, "\n")[tabBarRows]
}

func TestFirstRunWritesDefaultTags(t *testing.T) {
	h := startBoard(t)

	cfg := readConfig(t, h.configDir)
	if cfg.Version != 1 {
		t.Errorf("config version = %d, want 1", cfg.Version)
	}
	var names []string
	ids := map[string]bool{}
	for _, tag := range cfg.Tags {
		names = append(names, tag.Name)
		if tag.ID == "" || ids[tag.ID] {
			t.Errorf("tag %q has a missing or duplicate id %q", tag.Name, tag.ID)
		}
		ids[tag.ID] = true
		if tag.Color == "" {
			t.Errorf("tag %q has no color", tag.Name)
		}
		if tag.Terminal != (tag.Name == "Done") {
			t.Errorf("tag %q terminal = %v; only Done is terminal", tag.Name, tag.Terminal)
		}
	}
	if want := []string{"In Progress", "Review", "Testing", "Demo", "Done"}; !slices.Equal(names, want) {
		t.Errorf("default tags = %v, want %v", names, want)
	}
	if s, _ := readState(h.configDir); len(s.Assignments) > 0 {
		t.Errorf("assignments %v written before anything was moved", s.Assignments)
	}
}

func TestLanesRenderInTagOrderAfterUntagged(t *testing.T) {
	h := startBoard(t)

	header := headerLine(h.screen.plain())
	want := []string{"Untagged 3", "In Progress 0", "Review 0", "Testing 0", "Demo 0", "Done 0"}
	last := -1
	for _, w := range want {
		i := strings.Index(header, w)
		if i <= last {
			t.Fatalf("lane headers out of order or missing %q:\n%s", w, header)
		}
		last = i
	}
}

func TestHandEditedTagsAreReadOnStart(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", customConfig)
	h := startBoard(t, withConfigDir(dir))

	header := headerLine(h.screen.plain())
	if !strings.HasPrefix(header, "Untagged 3") || strings.Index(header, "Bravo 0") > strings.Index(header, "Alpha 0") {
		t.Errorf("lanes should be Untagged, Bravo, Alpha:\n%s", header)
	}
	if strings.Contains(header, "In Progress") {
		t.Errorf("default tags must not be added to an existing config:\n%s", header)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if string(raw) != customConfig {
		t.Errorf("an existing config must not be rewritten; now:\n%s", raw)
	}
}

func TestDefaultTagIDsAreStableAcrossRestarts(t *testing.T) {
	first := startBoard(t)
	before, _ := os.ReadFile(filepath.Join(first.configDir, "config.yaml"))
	first.press("q")
	first.waitFinished()

	startBoard(t, withConfigDir(first.configDir))
	after, _ := os.ReadFile(filepath.Join(first.configDir, "config.yaml"))
	if string(before) != string(after) {
		t.Errorf("config changed on restart:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestShiftKeysMoveTheSelectedCardOneLane(t *testing.T) {
	h := startBoard(t)
	tagID := func(name string) string {
		for _, tag := range readConfig(t, h.configDir).Tags {
			if tag.Name == name {
				return tag.ID
			}
		}
		t.Fatalf("no tag %q", name)
		return ""
	}

	h.press("L")
	h.waitForAssignments(map[string]string{newestPR: tagID("In Progress")})
	h.waitForText("Untagged 2")
	h.waitForText("In Progress 1")

	h.press(">")
	h.waitForAssignments(map[string]string{newestPR: tagID("Review")})
	h.waitForText("Review 1")

	h.press("<")
	h.waitForAssignments(map[string]string{newestPR: tagID("In Progress")})

	h.press("H")
	h.waitForAssignments(map[string]string{})
	h.waitForText("Untagged 3")
}

func TestMovesStopAtTheEdgeLanes(t *testing.T) {
	h := startBoard(t)

	h.press("H")
	for range 6 {
		h.press("L")
	}
	h.waitForText("Done 1")
	h.waitForAssignments(map[string]string{newestPR: "done"})
	if !strings.Contains(h.screen.plain(), "Untagged 2") {
		t.Errorf("moving past either edge should leave the card in place:\n%s", h.screen.plain())
	}
}

func TestVimAndArrowKeysChooseTheCardToMove(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		down, up, left, right, moveR string
	}{
		{name: "vim", down: "j", up: "k", left: "h", right: "l", moveR: "L"},
		{name: "arrows", down: "down", up: "up", left: "left", right: "right", moveR: ">"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := startBoard(t)

			h.press(tc.down)
			h.press(tc.down)
			h.press(tc.up)
			h.press(tc.moveR)
			h.waitForAssignments(map[string]string{olderPR: "in-progress"})

			h.press(tc.left)
			h.press(tc.down)
			h.press(tc.down)
			h.press(tc.moveR)
			h.waitForAssignments(map[string]string{olderPR: "in-progress", oldestPR: "in-progress"})

			h.press(tc.up)
			h.press(tc.moveR)
			h.waitForAssignments(map[string]string{olderPR: "review", oldestPR: "in-progress"})

			h.press(tc.left)
			h.press(tc.right)
			h.press(tc.left)
			h.press(tc.moveR)
			h.waitForAssignments(map[string]string{olderPR: "review", oldestPR: "review"})
		})
	}
}

func TestFocusFollowsAMovedCard(t *testing.T) {
	h := startBoard(t)

	h.press("j")
	h.press("L")
	h.press("L")
	h.waitForAssignments(map[string]string{olderPR: "review"})
}

func TestMoveToPickerJumpsACardToAnyLane(t *testing.T) {
	h := startBoard(t)

	h.press("m")
	screen := h.waitForText("Move to…")
	for _, lane := range []string{"Untagged", "In Progress", "Review", "Testing", "Demo", "Done"} {
		if strings.Count(screen, lane) < 2 {
			t.Errorf("picker should list %q:\n%s", lane, screen)
		}
	}
	h.press("j")
	h.press("j")
	h.press("j")
	h.press("enter")
	h.waitForAssignments(map[string]string{newestPR: "testing"})
	h.waitForScreen("the picker to close", func(s string) bool { return !strings.Contains(s, "Move to…") })

	h.press("m")
	h.waitForText("Move to…")
	h.press("k")
	h.press("k")
	h.press("k")
	h.press("enter")
	h.waitForAssignments(map[string]string{})
}

func TestMoveToPickerClosesOnEscWithoutMoving(t *testing.T) {
	h := startBoard(t)

	h.press("m")
	h.waitForText("Move to…")
	h.press("j")
	h.press("esc")
	h.waitForScreen("the picker to close", func(s string) bool { return !strings.Contains(s, "Move to…") })

	h.press("q")
	h.waitFinished()
	if s, _ := readState(h.configDir); len(s.Assignments) > 0 {
		t.Errorf("cancelling the picker must not move cards, got %v", s.Assignments)
	}
}

func TestPickerKeysDoNotReachTheBoard(t *testing.T) {
	h := startBoard(t)

	h.press("m")
	h.waitForText("Move to…")
	h.press("L")
	h.press("h")
	h.press("esc")
	h.waitForScreen("the picker to close", func(s string) bool { return !strings.Contains(s, "Move to…") })
	h.press("q")
	h.waitFinished()
	if s, _ := readState(h.configDir); len(s.Assignments) > 0 {
		t.Errorf("keys pressed in the picker must not move cards, got %v", s.Assignments)
	}
}

func TestAssignmentsSurviveARestart(t *testing.T) {
	first := startBoard(t)
	first.press("j")
	first.press("L")
	first.press("L")
	first.press("q")
	first.waitFinished()

	h := startBoard(t, withConfigDir(first.configDir))
	h.waitForText("Review 1")
	if header := headerLine(h.screen.plain()); !strings.Contains(header, "Untagged 2") {
		t.Errorf("the moved card should still be in Review after a restart:\n%s", header)
	}
	h.waitForAssignments(map[string]string{olderPR: "review"})
}

func TestQuittingRightAfterAMoveStillSavesIt(t *testing.T) {
	h := startBoard(t)

	h.press("L")
	h.press("q")
	h.waitFinished()
	s, ok := readState(h.configDir)
	if !ok || s.Assignments[newestPR] != "in-progress" {
		t.Errorf("state after quitting = %+v, want %s in in-progress", s, newestPR)
	}
}

func TestSeededAssignmentsPlaceCardsInTheirLanes(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "config.yaml", customConfig)
	seedFile(t, dir, "state.json", `{"version":1,"assignments":{"PR_node_older":"tag-a","PR_node_newer":"tag-gone"}}`)
	h := startBoard(t, withConfigDir(dir))

	header := headerLine(h.waitForText("Alpha 1"))
	for _, want := range []string{"Untagged 2", "Bravo 0", "Alpha 1"} {
		if !strings.Contains(header, want) {
			t.Errorf("header is missing %q; a PR whose tag is gone belongs in Untagged:\n%s", want, header)
		}
	}
}

func TestWritesLeaveNoTempFilesBehind(t *testing.T) {
	h := startBoard(t)
	h.press("L")
	h.press("L")
	h.waitForAssignments(map[string]string{newestPR: "review"})
	h.press("q")
	h.waitFinished()

	entries, err := os.ReadDir(h.configDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"config.yaml", "state.json"}; !slices.Equal(names, want) {
		t.Errorf("config dir holds %v, want %v", names, want)
	}
}

func TestUnreadableConfigIsReportedAndNothingIsWritten(t *testing.T) {
	for _, tc := range []struct {
		name, file, content, want string
	}{
		{name: "malformed config", file: "config.yaml", content: "tags: [\n", want: "read config"},
		{name: "newer config", file: "config.yaml", content: "version: 2\ntags: []\n", want: "unsupported version 2"},
		{name: "duplicate tag ids", file: "config.yaml", content: "version: 1\ntags:\n  - {id: x, name: A}\n  - {id: x, name: B}\n", want: `"x" is used more than once`},
		{name: "malformed state", file: "state.json", content: "{not json", want: "read state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			seedFile(t, dir, tc.file, tc.content)
			h := newHarness(t, boardTransport(t), withConfigDir(dir))

			screen := h.waitForText("octo-org/sample-repo#7")
			screen = h.waitForText(tc.want)
			if !strings.Contains(screen, "Couldn't load tags") {
				t.Errorf("status bar should say the tags couldn't load:\n%s", screen)
			}
			if !onScreen(screen, "octo-org/sample-repo#7") {
				t.Errorf("the pull requests should still be shown:\n%s", screen)
			}
			h.press("L")
			h.press("q")
			h.waitFinished()

			raw, _ := os.ReadFile(filepath.Join(dir, tc.file))
			if string(raw) != tc.content {
				t.Errorf("%s was rewritten:\n%s", tc.file, raw)
			}
			if tc.file == "config.yaml" {
				if _, ok := readState(dir); ok {
					t.Errorf("a move with an unreadable config must not write state")
				}
			}
		})
	}
}
