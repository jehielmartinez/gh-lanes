// Package markdown lays out GitHub-flavoured markdown for a terminal. It
// returns lines of styled spans rather than a styled string, so the caller
// knows the line and column of every link it draws, wrapped ones included.
// An off-the-shelf renderer emits finished ANSI text and loses where links
// landed, which is why this one exists and why it stays small.
package markdown

import (
	"regexp"
	"slices"
	"strings"
)

// Style is how a span is drawn, as a set of flags.
type Style int

// The styles a span can carry.
const (
	Bold Style = 1 << iota
	Italic
	Strike
	Code
	Heading
	// Muted marks decoration the renderer adds, such as list bullets, quote
	// bars and rules.
	Muted
)

// Span is a run of text drawn in one style.
type Span struct {
	Text  string
	Style Style
	// URL is the link the text belongs to, empty for plain text.
	URL string
}

// Line is one rendered line, no wider than the width it was rendered at.
type Line []Span

// minWidth keeps layout possible on an absurdly narrow screen.
const minWidth = 8

// Render lays out markdown source at width columns.
func Render(src string, width int) []Line {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = htmlComment.ReplaceAllString(src, "")
	return renderBlocks(strings.Split(src, "\n"), max(width, minWidth))
}

// htmlComment matches the HTML comments PR templates are full of, which
// GitHub hides.
var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

var (
	fenceOpen  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	atxHeading = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$`)
	rule       = regexp.MustCompile(`^ {0,3}(?:(?:-[ \t]*){3,}|(?:\*[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	setext     = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	quoteLine  = regexp.MustCompile(`^ {0,3}> ?(.*)$`)
	listItem   = regexp.MustCompile(`^( *)([-*+]|\d{1,9}[.)])([ \t]+|$)(.*)$`)
	tableRow   = regexp.MustCompile(`^ {0,3}\|`)
	tableRule  = regexp.MustCompile(`^ {0,3}\|?[ \t]*:?-+:?[ \t]*(\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$`)
)

// block is one rendered block. A list item follows the block before it
// directly unless the source had a blank line between them; every other pair
// of blocks gets a blank line. A block that rendered to nothing, such as a
// lone HTML tag, takes no space.
type block struct {
	lines    []Line
	listItem bool
	// gap is whether a blank line came before the block in the source.
	gap bool
}

// renderBlocks splits lines into blocks and renders each. It is one loop
// over the lines because markdown's block rules are decided by what starts
// the next line, and splitting that decision up would scatter it.
func renderBlocks(lines []string, width int) []Line {
	var blocks []block
	var para []string
	sawBlank, paraGap := false, false
	add := func(b block) {
		b.gap = sawBlank
		sawBlank = false
		blocks = append(blocks, b)
	}
	flush := func() {
		if len(para) > 0 {
			blocks = append(blocks, block{lines: wrap(parseInline(strings.Join(para, " "), 0, ""), width), gap: paraGap})
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			flush()
			sawBlank = true
		case fenceOpen.MatchString(line):
			flush()
			var code []string
			code, i = fenced(lines, i)
			add(block{lines: codeBlock(code, width)})
		case len(para) == 0 && indentOf(line) >= 4:
			var code []string
			for ; i < len(lines) && (indentOf(lines[i]) >= 4 || strings.TrimSpace(lines[i]) == ""); i++ {
				code = append(code, strings.TrimPrefix(expandTabs(lines[i]), "    "))
			}
			i--
			add(block{lines: codeBlock(trimBlank(code), width)})
		case atxHeading.MatchString(line):
			flush()
			text := atxHeading.FindStringSubmatch(line)[2]
			add(block{lines: wrap(parseInline(text, Heading|Bold, ""), width)})
		case len(para) > 0 && setext.MatchString(line):
			text := strings.Join(para, " ")
			para = nil
			blocks = append(blocks, block{lines: wrap(parseInline(text, Heading|Bold, ""), width), gap: paraGap})
		case rule.MatchString(line):
			flush()
			add(block{lines: []Line{{{Text: strings.Repeat("─", width), Style: Muted}}}})
		case quoteLine.MatchString(line):
			flush()
			var inner []string
			for ; i < len(lines) && quoteLine.MatchString(lines[i]); i++ {
				inner = append(inner, quoteLine.FindStringSubmatch(lines[i])[1])
			}
			i--
			bar := Span{Text: "│ ", Style: Muted}
			add(block{lines: prefix(renderBlocks(inner, width-2), bar, bar)})
		case listItem.MatchString(line):
			flush()
			var item []Line
			item, i = listBlock(lines, i, width)
			add(block{lines: item, listItem: true})
		case tableRow.MatchString(line):
			flush()
			var rows []Line
			for ; i < len(lines) && tableRow.MatchString(lines[i]); i++ {
				if !tableRule.MatchString(lines[i]) {
					rows = append(rows, wrap(parseInline(strings.TrimSpace(lines[i]), 0, ""), width)...)
				}
			}
			i--
			add(block{lines: rows})
		default:
			if len(para) == 0 {
				paraGap, sawBlank = sawBlank, false
			}
			para = append(para, strings.TrimSpace(line))
		}
	}
	flush()
	return join(blocks)
}

func join(blocks []block) []Line {
	var out []Line
	var prev *block
	for i, b := range blocks {
		if len(b.lines) == 0 {
			continue
		}
		if prev != nil && (b.gap || !b.listItem) {
			out = append(out, Line{})
		}
		out = append(out, b.lines...)
		prev = &blocks[i]
	}
	return out
}

// fenced returns the code inside the fence opening at lines[start] and the
// index of its closing line. An unclosed fence runs to the end, as GitHub
// draws it.
func fenced(lines []string, start int) ([]string, int) {
	marker := fenceOpen.FindStringSubmatch(lines[start])[1]
	indent := indentOf(lines[start])
	var code []string
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, marker) && strings.Trim(trimmed, marker[:1]) == "" {
			return code, i
		}
		code = append(code, trimLeadingSpaces(expandTabs(lines[i]), indent))
	}
	return code, len(lines)
}

// codeBlock draws code as it was written, indented by two columns and cut
// into pieces where a line is wider than the screen.
func codeBlock(code []string, width int) []Line {
	var out []Line
	for _, l := range code {
		pieces := hardWrap(Line{{Text: expandTabs(l), Style: Code}}, width-2)
		out = append(out, prefix(pieces, Span{Text: "  "}, Span{Text: "  "})...)
	}
	return out
}

// listBlock renders the list item starting at lines[start], with everything
// indented beneath it, and returns the index of its last line.
func listBlock(lines []string, start, width int) ([]Line, int) {
	m := listItem.FindStringSubmatch(expandTabs(lines[start]))
	indent, marker, gap, text := len(m[1]), m[2], m[3], m[4]
	content := indent + len(marker) + max(len(gap), 1)
	if len(gap) > 4 {
		content = indent + len(marker) + 1
	}
	body := []string{text}
	end := start
	for i := start + 1; i < len(lines); i++ {
		line := expandTabs(lines[i])
		blank := strings.TrimSpace(line) == ""
		switch {
		case blank:
			if next := nextNonBlank(lines, i); next < 0 || indentOf(lines[next]) < content {
				return renderItem(marker, body, width), end
			}
			body = append(body, "")
		case indentOf(line) >= content:
			body = append(body, line[content:])
		case startsBlock(line) || body[len(body)-1] == "":
			return renderItem(marker, body, width), end
		default:
			body = append(body, strings.TrimSpace(line))
		}
		end = i
	}
	return renderItem(marker, body, width), end
}

func renderItem(marker string, body []string, width int) []Line {
	body = slices.Clone(body)
	bullet := marker
	if strings.ContainsAny(marker, "-*+") {
		bullet = "•"
	}
	switch {
	case strings.HasPrefix(body[0], "[ ] "):
		bullet, body[0] = "☐", body[0][4:]
	case strings.HasPrefix(body[0], "[x] "), strings.HasPrefix(body[0], "[X] "):
		bullet, body[0] = "☑", body[0][4:]
	}
	gutter := len([]rune(bullet)) + 1
	first := Span{Text: bullet + " ", Style: Muted}
	return prefix(renderBlocks(body, width-gutter), first, Span{Text: strings.Repeat(" ", gutter)})
}

// startsBlock is whether a line opens a block of its own rather than
// continuing a paragraph above it.
func startsBlock(line string) bool {
	return fenceOpen.MatchString(line) || atxHeading.MatchString(line) || rule.MatchString(line) ||
		quoteLine.MatchString(line) || listItem.MatchString(line) || tableRow.MatchString(line)
}

func nextNonBlank(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}

// prefix puts first before the first line and rest before every other.
func prefix(lines []Line, first, rest Span) []Line {
	if len(lines) == 0 {
		return []Line{{first}}
	}
	out := make([]Line, len(lines))
	for i, l := range lines {
		p := rest
		if i == 0 {
			p = first
		}
		out[i] = append(Line{p}, l...)
	}
	return out
}

func indentOf(line string) int {
	line = expandTabs(line)
	return len(line) - len(strings.TrimLeft(line, " "))
}

func trimLeadingSpaces(s string, n int) string {
	for i := 0; i < n && strings.HasPrefix(s, " "); i++ {
		s = s[1:]
	}
	return s
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
