package ui_test

import (
	"strings"
	"testing"
)

// Cards of search_title_pattern.json by reference, most recently updated
// first, with their titles:
//
//	t1 "sup-1234 add retries"
//	t2 "Bump OPS-7 before SUP-1234 and #42"
//	t3 "SUP-1234 fix login"
//	t4 "Fix #7 crash in the parser"
//	t5 "Add a health check"
//	t6 "SUP-88 #7 tidy the build"
const (
	t1Ref = "octo-org/sample-repo#31"
	t2Ref = "user-a/sample-repo#32"
	t3Ref = "octo-org/other-repo#33"
	t4Ref = "user-a/other-repo#34"
	t5Ref = "octo-org/sample-repo#35"
	t6Ref = "user-a/sample-repo#36"
)

// titlePatternConfig is the default tags with Title pattern grouping and,
// unless it is empty, the title_pattern line given.
func titlePatternConfig(patternLine string) string {
	return groupingConfig("grouping: title_pattern\n" + patternLine)
}

func startTitlePattern(t *testing.T, config string, fixtures ...string) *harness {
	t.Helper()
	if len(fixtures) == 0 {
		fixtures = []string{"search_title_pattern.json"}
	}
	return startGrouped(t, config, fixtures...)
}

func TestTitlePatternDefaultGroupsJiraKeysWithNoMatchLast(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(""))
	screen := h.waitForText("SUP-1234 (1)")

	assertTopToBottom(t, screen,
		"OPS-7 (1)", t2Ref,
		"SUP-1234 (1)", t3Ref,
		"SUP-88 (1)", t6Ref,
		"No match (3)", t1Ref, t4Ref, t5Ref,
	)
	assertContains(t, headerLine(screen), "Untagged 6")
	assertContains(t, screen, "grouped by title pattern")
}

func TestTitlePatternCustomPatternGroupsByItsMatches(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(`title_pattern: '#\d+'`+"\n"))
	screen := h.waitForText("#7 (2)")

	assertTopToBottom(t, screen,
		"#42 (1)", t2Ref,
		"#7 (2)", t4Ref, t6Ref,
		"No match (3)", t1Ref, t3Ref, t5Ref,
	)
	assertContains(t, headerLine(screen), "Untagged 6")
}

func TestTitlePatternAlternationGroupsByTheLeftmostMatch(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(`title_pattern: 'SUP-\d+|#\d+'`+"\n"))
	screen := h.waitForText("SUP-1234 (2)")

	assertTopToBottom(t, screen,
		"SUP-1234 (2)", t2Ref, t3Ref,
		"#7 (1)", t4Ref,
		"SUP-88 (1)", t6Ref,
		"No match (2)", t1Ref, t5Ref,
	)
	for _, absent := range []string{"#42 (", "OPS-7 ("} {
		if strings.Contains(screen, absent) {
			t.Errorf("a later match named a group, %q:\n%s", absent, screen)
		}
	}
	assertContains(t, headerLine(screen), "Untagged 6")
}

func TestTitlePatternMergesCaseVariantsUnderTheNewestSpelling(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(`title_pattern: '(?i)sup-\d+'`+"\n"))
	screen := h.waitForText("sup-1234 (3)")

	assertTopToBottom(t, screen,
		"sup-1234 (3)", t1Ref, t2Ref, t3Ref,
		"SUP-88 (1)", t6Ref,
		"No match (2)", t4Ref, t5Ref,
	)
	if strings.Contains(screen, "SUP-1234 (") {
		t.Errorf("SUP-1234 should merge into sup-1234, the newest card's spelling:\n%s", screen)
	}
}

func TestTitlePatternRegroupsARetitledPullRequestOnRefresh(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(""), "search_title_pattern.json", "search_title_pattern_retitled.json")
	h.waitForText("No match (3)")

	h.press("r")
	screen := h.waitForText("OPS-7 (2)")

	assertTopToBottom(t, screen, "OPS-7 (2)", t2Ref, t5Ref, "No match (2)", t1Ref, t4Ref)
}

func TestTitlePatternHeaderNeverCarriesEscapeSequences(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig(`title_pattern: '^\S+'`+"\n"), "search_title_pattern_hostile.json")
	h.waitForText("OPS-7 (1)")

	raw := h.screen.content()
	for _, bad := range []string{"\x1b[2J", "\x1b]0;", "pwned", "\u009b", "\x07"} {
		if strings.Contains(raw, bad) {
			t.Errorf("rendered screen contains %q from the PR title", bad)
		}
	}
}

func TestInvalidTitlePatternFallsBackToTheDefaultAndKeepsTheConfig(t *testing.T) {
	config := titlePatternConfig("title_pattern: '[A-Z'\n")
	h := startTitlePattern(t, config)
	screen := h.waitForText("SUP-1234 (1)")

	assertTopToBottom(t, screen, "OPS-7 (1)", "SUP-1234 (1)", "SUP-88 (1)", "No match (3)")
	assertContains(t, screen, `title_pattern "[A-Z"`, "grouped by title pattern")
	if got := readFile(t, h.configDir, "config.yaml"); got != config {
		t.Errorf("config was rewritten:\n%s\nwant:\n%s", got, config)
	}
}

func TestInvalidTitlePatternSurvivesASaveFromTheTagManager(t *testing.T) {
	h := startTitlePattern(t, titlePatternConfig("title_pattern: '[A-Z'\n"))
	h.waitForText(`title_pattern "[A-Z"`)

	h.press("t")
	h.waitForText("terminal")
	h.press("j")
	h.press("t")
	h.press("esc")
	h.waitFor("the tag manager's save", func() bool {
		return strings.Count(readFile(t, h.configDir, "config.yaml"), "terminal: true") == 2
	})
	screen := h.waitForText("SUP-1234 (1)")

	assertContains(t, screen, `title_pattern "[A-Z"`)
	assertContains(t, readFile(t, h.configDir, "config.yaml"), "title_pattern: '[A-Z'", "grouping: title_pattern")
}
