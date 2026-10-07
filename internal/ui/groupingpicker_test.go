package ui_test

import (
	"maps"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// pickerHint is in the grouping picker's help line, and nowhere else.
const pickerHint = "e edit pattern"

// patternEditHint is in the help line while the title pattern is edited.
const patternEditHint = "enter save"

// defaultPattern is the title pattern a config without one uses.
const defaultPattern = `[A-Z][A-Z0-9]+-\d+`

// openGroupingPicker presses g and waits for the grouping picker.
func (h *harness) openGroupingPicker() string {
	h.t.Helper()
	h.press("g")
	return h.waitForText(pickerHint)
}

// closedPicker reports whether the grouping picker is off the screen.
func closedPicker(s string) bool { return !strings.Contains(s, pickerHint) }

// configMap is the config file in dir as plain YAML values.
func configMap(t *testing.T, dir string) map[string]any {
	t.Helper()
	return yamlMap(t, readFile(t, dir, "config.yaml"))
}

func yamlMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("config is not YAML: %v\n%s", err, raw)
	}
	return m
}

// assertConfigIs checks the config file in dir holds what config does, with
// the settings in set replaced and nothing else changed.
func assertConfigIs(t *testing.T, dir, config string, set map[string]any) {
	t.Helper()
	want := yamlMap(t, config)
	maps.Copy(want, set)
	if got := configMap(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("config is\n%v\nwant\n%v", got, want)
	}
}

// waitForSetting waits until the config file's key holds value.
func (h *harness) waitForSetting(key string, value any) {
	h.t.Helper()
	h.waitFor("the config to set "+key, func() bool {
		var m map[string]any
		return yaml.Unmarshal([]byte(readFile(h.t, h.configDir, "config.yaml")), &m) == nil && reflect.DeepEqual(m[key], value)
	})
}

func TestGOpensTheGroupingPickerOnEveryTab(t *testing.T) {
	h := startGroupedLists(t, groupingConfig(""), groupingArchivedState, groupingTermHeight)

	for i, tab := range []string{"Board", "Review requests", "Archived"} {
		if i > 0 {
			h.press("tab")
		}
		screen := h.openGroupingPicker()
		assertContains(t, screen, "Group by", "None (current)", "Owner", "Repository", "Title pattern", defaultPattern)
		assertTopToBottom(t, screen, "None (current)", "Owner", "Repository", "Title pattern", defaultPattern)
		h.press("esc")
		h.waitForScreen("the picker to close on "+tab, closedPicker)
	}
}

func TestGroupingPickerMarksTheModeAndPatternInUse(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(`title_pattern: '#\d+'`+"\n"))
	h.waitForText("#7 (2)")

	screen := h.openGroupingPicker()
	assertContains(t, screen, "Title pattern (current)", `#\d+`)
	if strings.Contains(screen, "None (current)") {
		t.Errorf("None is marked current:\n%s", screen)
	}
}

func TestGInTheDetailModalStillJumpsToTheTop(t *testing.T) {
	h := newModalHarness(t, "detail_conflict.json")
	h.resize(120, 24)
	h.doubleClickCard("octo-org/sample-repo#11")
	h.waitForText("retry-uploads → release-2")
	h.press("end")
	h.waitForScreen("the modal to scroll down", func(s string) bool { return !strings.Contains(s, "retry-uploads → release-2") })

	h.press("g")
	screen := h.waitForText("retry-uploads → release-2")
	if !closedPicker(screen) {
		t.Errorf("g in the modal opened the grouping picker:\n%s", screen)
	}
}

func TestChoosingAModeRegroupsEveryTabAndSavesOnlyTheGrouping(t *testing.T) {
	for _, tc := range []struct {
		down     int
		written  string
		board    []string
		reviews  []string
		archived []string
	}{
		{1, "owner",
			[]string{"octo-org (3)", g1Ref, "user-a (2)", g2Ref},
			[]string{"octo-org (3)", r1Ref, "user-b (2)", r2Ref},
			[]string{"octo-org (2)", mergedArchivedRef, openArchivedRef, "user-a (1)", closedArchivedRef}},
		{2, "repository",
			[]string{"octo-org/sample-repo (2)", g1Ref, "user-a/sample-repo (1)", g2Ref},
			[]string{"octo-org/review-repo (2)", r1Ref, "user-b/sample-repo (1)", r2Ref},
			[]string{"octo-org/sample-repo (1)", mergedArchivedRef, "user-a/other-repo (1)", closedArchivedRef}},
		{3, "title_pattern",
			[]string{"SUP-1234 (1)", g1Ref, "No match (4)", g2Ref},
			[]string{"No match (5)", r1Ref, r2Ref},
			[]string{"No match (3)", mergedArchivedRef, closedArchivedRef}},
	} {
		t.Run(tc.written, func(t *testing.T) {
			config := groupingConfig("refresh_interval: 5m\n")
			h := startGroupedLists(t, config, groupingArchivedState, groupingTermHeight)
			h.waitForSelected(g1Ref)

			h.openGroupingPicker()
			for range tc.down {
				h.press("j")
			}
			h.press("enter")
			h.waitForSetting("grouping", tc.written)
			screen := h.waitForScreen("the board regrouped", func(s string) bool {
				return closedPicker(s) && onScreen(s, tc.board[0])
			})

			assertTopToBottom(t, screen, tc.board...)
			assertContains(t, screen, "grouped by "+strings.ReplaceAll(tc.written, "_", " "))
			assertContains(t, headerLine(screen), "Untagged 5")
			assertConfigIs(t, h.configDir, config, map[string]any{"grouping": tc.written})

			screen = h.openReviewRequests()
			assertTopToBottom(t, screen, tc.reviews...)
			h.press("tab")
			screen = h.waitForSelected(mergedArchivedRef)
			assertTopToBottom(t, screen, tc.archived...)
		})
	}
}

func TestChoosingNoneUngroupsAndSavesIt(t *testing.T) {
	config := groupingConfig("grouping: owner\n")
	h := startGrouped(t, config)
	h.waitForText("octo-org (3)")

	screen := h.openGroupingPicker()
	assertContains(t, screen, "Owner (current)")
	h.press("k")
	h.press("enter")
	h.waitForSetting("grouping", "none")
	screen = h.waitForScreen("the groups to go", func(s string) bool { return !onScreen(s, "octo-org (3)") })

	assertTopToBottom(t, screen, g1Ref, g2Ref, g3Ref, g4Ref, g5Ref)
	if strings.Contains(screen, "grouped by") {
		t.Errorf("status bar still names a grouping:\n%s", screen)
	}
	assertConfigIs(t, h.configDir, config, map[string]any{"grouping": "none"})
}

func TestChangingTheGroupingKeepsTheSelectedCard(t *testing.T) {
	h := startGrouped(t, groupingConfig(""))
	h.waitForSelected(g1Ref)
	h.press("j")
	h.press("j")
	h.waitForSelected(g3Ref)

	h.openGroupingPicker()
	h.press("j")
	h.press("j")
	h.press("enter")
	h.waitForText("octo-org/other-repo (1)")
	screen := h.waitForSelected(g3Ref)
	assertTopToBottom(t, screen, "octo-org/other-repo (1)", g3Ref)

	h.openGroupingPicker()
	h.press("j")
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press("build")
	h.press("enter")
	h.waitForSetting("title_pattern", "build")
	h.press("enter")
	h.waitForSetting("grouping", "title_pattern")
	h.waitForText("No match (4)")
	screen = h.waitForSelected(g3Ref)
	assertTopToBottom(t, screen, "build (1)", g3Ref, "No match (4)")
}

func TestEditingThePatternStartsFromTheOneInUse(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(`title_pattern: '#\d+'`+"\n"))
	h.waitForText("#7 (2)")

	h.openGroupingPicker()
	h.press("e")
	screen := h.waitForText(patternEditHint)
	assertContains(t, screen, `> #\d+`)
}

func TestEOnlyEditsOnTitlePattern(t *testing.T) {
	h := startGrouped(t, groupingConfig(""))
	h.openGroupingPicker()

	h.press("e")
	h.press("j")
	screen := h.waitForText("› Owner")
	if strings.Contains(screen, patternEditHint) {
		t.Errorf("e on None opened the pattern editor:\n%s", screen)
	}
}

func TestSavingAValidPatternRegroupsAndSavesOnlyThePattern(t *testing.T) {
	config := titlePatternConfig("")
	h := startTitlePattern(t, config)
	h.waitForText("SUP-1234 (1)")

	h.openGroupingPicker()
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press(`#\d+`)
	h.waitForText(`> #\d+`)
	assertContains(t, h.waitForText("valid pattern"), `> #\d+`)
	h.press("enter")
	h.waitForSetting("title_pattern", `#\d+`)
	screen := h.waitForText(`› Title pattern (current)`)
	assertContains(t, screen, `#\d+`)
	h.press("esc")
	screen = h.waitForScreen("the picker to close", closedPicker)

	assertTopToBottom(t, screen,
		"#42 (1)", t2Ref,
		"#7 (2)", t4Ref, t6Ref,
		"No match (3)", t1Ref, t3Ref, t5Ref,
	)
	assertConfigIs(t, h.configDir, config, map[string]any{"title_pattern": `#\d+`})
}

func TestAnInvalidPatternCantBeSaved(t *testing.T) {
	config := titlePatternConfig("")
	h := startTitlePattern(t, config)
	h.waitForText("SUP-1234 (1)")

	h.openGroupingPicker()
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press("[A-Z")
	screen := h.waitForText("> [A-Z")
	assertContains(t, h.waitForText("missing closing ]"), "> [A-Z")
	h.press("enter")
	h.press("]")
	h.waitForText("valid pattern")
	h.press("esc")
	screen = h.waitForText(pickerHint)
	assertContains(t, screen, defaultPattern)
	h.press("esc")
	screen = h.waitForScreen("the picker to close", closedPicker)

	assertTopToBottom(t, screen, "OPS-7 (1)", "SUP-1234 (1)", "SUP-88 (1)", "No match (3)")
	if got := readFile(t, h.configDir, "config.yaml"); got != config {
		t.Errorf("config was written:\n%s\nwant:\n%s", got, config)
	}
}

func TestAnEmptyPatternCantBeSaved(t *testing.T) {
	config := titlePatternConfig("")
	h := startTitlePattern(t, config)
	h.waitForText("SUP-1234 (1)")

	h.openGroupingPicker()
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.waitForText("The pattern is empty")
	h.press("enter")
	h.press("esc")
	h.waitForText(pickerHint)
	h.press("esc")
	h.waitForScreen("the picker to close", closedPicker)

	if got := readFile(t, h.configDir, "config.yaml"); got != config {
		t.Errorf("config was written:\n%s\nwant:\n%s", got, config)
	}
}

func TestEscCancelsAnEditThenClosesThePicker(t *testing.T) {
	config := groupingConfig("")
	h := startGrouped(t, config)
	h.waitForSelected(g1Ref)

	h.openGroupingPicker()
	h.press("j")
	h.press("j")
	h.press("j")
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press("other")
	h.press("esc")
	screen := h.waitForText(pickerHint)
	assertContains(t, screen, "› Title pattern", defaultPattern)
	h.press("esc")
	screen = h.waitForScreen("the picker to close", closedPicker)

	assertTopToBottom(t, screen, g1Ref, g2Ref, g3Ref, g4Ref, g5Ref)
	if got := readFile(t, h.configDir, "config.yaml"); got != config {
		t.Errorf("config was written:\n%s\nwant:\n%s", got, config)
	}
}

func TestLettersTypedIntoThePatternAreText(t *testing.T) {
	h := startGrouped(t, groupingConfig("grouping: title_pattern\n"))
	h.openGroupingPicker()
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press("qgjk?")
	h.press("enter")
	h.waitForSetting("title_pattern", "qgjk?")
}

func TestSavingAPatternClearsTheConfigsPatternError(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig("title_pattern: '[A-Z'\n"))
	h.waitForText(`title_pattern "[A-Z"`)

	h.openGroupingPicker()
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press(`SUP-\d+`)
	h.press("enter")
	h.waitForSetting("title_pattern", `SUP-\d+`)
	h.press("esc")
	screen := h.waitForScreen("the error to clear", func(s string) bool {
		return !strings.Contains(s, `title_pattern "[A-Z"`) && closedPicker(s)
	})

	assertTopToBottom(t, screen, "SUP-1234 (2)", "SUP-88 (1)", "No match (3)")
}

func TestGroupingAndPatternSurviveARestart(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(""))
	h.waitForText("SUP-1234 (1)")
	h.openGroupingPicker()
	h.press("e")
	h.waitForText(patternEditHint)
	h.press("ctrl+u")
	h.press(`#\d+`)
	h.press("enter")
	h.waitForSetting("title_pattern", `#\d+`)
	h.press("k")
	h.press("enter")
	h.waitForSetting("grouping", "repository")

	transport := groupingTransport(t, "search_title_pattern.json")
	restarted := newHarness(t, transport, withConfigDir(h.configDir), withTermSize(defaultTermWidth, groupingTermHeight))
	restarted.waitForText("octo-org/sample-repo (2)")
	screen := restarted.openGroupingPicker()
	assertContains(t, screen, "Repository (current)", `#\d+`)
}

// fullHelpGrouping is the grouping picker's row in the full help, where
// keys and descriptions are columns apart.
var fullHelpGrouping = regexp.MustCompile(`g\s+grouping`)

func inFullHelp(s string) bool {
	return strings.Contains(s, "close help") && fullHelpGrouping.MatchString(s)
}

func TestHelpListsTheGroupingPicker(t *testing.T) {
	h := startGroupedLists(t, groupingConfig(""), "", groupingTermHeight)
	h.waitForSelected(g1Ref)

	assertContains(t, h.waitForText("g grouping"), "g grouping")
	h.press("?")
	h.waitForScreen("g in the full help", inFullHelp)
	h.press("?")
	h.waitForText("? help")
	h.openReviewRequests()
	h.waitForText("g grouping")
	h.press("?")
	h.waitForScreen("g in the full help", inFullHelp)
}

func TestAHandEditAfterAChoiceIsKeptByTheNextSave(t *testing.T) {
	h := startGrouped(t, groupingConfig(""))
	h.openGroupingPicker()
	h.press("j")
	h.press("enter")
	h.waitForSetting("grouping", "owner")
	h.waitForText("octo-org (3)")

	seedFile(t, h.configDir, "config.yaml", groupingConfig("grouping: repository\ntitle_pattern: 'x+'\n"))
	h.press("t")
	h.waitForText("terminal")
	h.press("j")
	h.press("t")
	h.press("esc")
	h.waitForText("grouped by repository")

	assertContains(t, readFile(t, h.configDir, "config.yaml"), "grouping: repository", "title_pattern: x+")
}
