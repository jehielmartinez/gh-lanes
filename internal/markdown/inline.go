package markdown

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

var (
	bareURL  = regexp.MustCompile(`^https?://[^\s<>]+`)
	autolink = regexp.MustCompile(`^<(https?://[^\s<>]+)>`)
	// htmlTag matches the tags GitHub renders as layout rather than text;
	// any other angle-bracketed text is kept, since it is more often a type
	// parameter or a placeholder than markup.
	htmlTag = regexp.MustCompile(`(?i)^</?(details|summary|br|p|div|span|sub|sup|kbd|b|i|em|strong|img|picture|source)\b[^>]*>`)
)

// parseInline turns one paragraph's text into spans, adding style to all of
// them and url to any that aren't a link of their own.
func parseInline(s string, style Style, url string) []Span {
	var spans []Span
	var text strings.Builder
	emit := func(sp ...Span) {
		if text.Len() > 0 {
			spans = appendSpan(spans, Span{Text: text.String(), Style: style, URL: url})
			text.Reset()
		}
		for _, x := range sp {
			spans = appendSpan(spans, x)
		}
	}
	for i := 0; i < len(s); {
		rest := s[i:]
		if sp, n, ok := inlineToken(s, i, style, url); ok {
			emit(sp...)
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(rest)
		text.WriteRune(r)
		i += size
	}
	emit()
	return spans
}

// inlineToken recognises the construct starting at s[i], if any, and returns
// its spans and length in bytes.
func inlineToken(s string, i int, style Style, url string) ([]Span, int, bool) {
	rest := s[i:]
	switch c := rest[0]; {
	case c == '\\' && len(rest) > 1 && isASCIIPunct(rest[1]):
		return []Span{{Text: rest[1:2], Style: style, URL: url}}, 2, true
	case c == '`':
		return codeSpan(rest, style, url)
	case c == '!' && strings.HasPrefix(rest, "!["):
		if text, target, n, ok := link(rest[1:]); ok {
			if text == "" {
				text = "image"
			}
			return parseInline(text, style, orURL(url, target)), n + 1, true
		}
	case c == '[':
		if text, target, n, ok := link(rest); ok {
			return parseInline(text, style, orURL(url, target)), n, true
		}
	case c == '<':
		if m := autolink.FindStringSubmatch(rest); m != nil {
			return []Span{{Text: m[1], Style: style, URL: orURL(url, m[1])}}, len(m[0]), true
		}
		if m := htmlTag.FindStringSubmatch(rest); m != nil {
			if strings.EqualFold(m[1], "br") {
				return []Span{{Text: " ", Style: style, URL: url}}, len(m[0]), true
			}
			return nil, len(m[0]), true
		}
	case c == 'h' && url == "" && (i == 0 || !isWordByte(s[i-1])):
		if m := bareURL.FindString(rest); m != "" {
			m = trimURLTail(m)
			return []Span{{Text: m, Style: style, URL: m}}, len(m), true
		}
	case c == '*' || c == '_' || c == '~':
		return emphasis(s, i, style, url)
	}
	return nil, 0, false
}

func orURL(outer, inner string) string {
	if outer != "" {
		return outer
	}
	return inner
}

// codeSpan matches a backtick run with a closing run of the same length.
func codeSpan(rest string, style Style, url string) ([]Span, int, bool) {
	ticks := len(rest) - len(strings.TrimLeft(rest, "`"))
	delim := rest[:ticks]
	end := strings.Index(rest[ticks:], delim)
	for end >= 0 && strings.HasPrefix(rest[ticks+end+ticks:], "`") {
		next := strings.Index(rest[ticks+end+ticks:], delim)
		if next < 0 {
			end = -1
			break
		}
		end += ticks + next
	}
	if end < 0 {
		return []Span{{Text: delim, Style: style, URL: url}}, ticks, true
	}
	code := rest[ticks : ticks+end]
	if len(code) > 1 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.TrimSpace(code) != "" {
		code = code[1 : len(code)-1]
	}
	return []Span{{Text: code, Style: style | Code, URL: url}}, ticks + end + ticks, true
}

// link matches [text](target "title") at the start of s, returning the text,
// the target and the length matched.
func link(s string) (text, target string, n int, ok bool) {
	depth := 0
	closeText := -1
	for i := 0; i < len(s) && closeText < 0; i++ {
		switch s[i] {
		case '\\':
			i++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				closeText = i
			}
		}
	}
	if closeText < 0 || closeText+1 >= len(s) || s[closeText+1] != '(' {
		return "", "", 0, false
	}
	depth = 0
	for i := closeText + 1; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				dest := strings.TrimSpace(s[closeText+2 : i])
				if sp := strings.IndexAny(dest, " \t"); sp >= 0 {
					dest = dest[:sp]
				}
				dest = strings.TrimSuffix(strings.TrimPrefix(dest, "<"), ">")
				return s[1:closeText], dest, i + 1, true
			}
		}
	}
	return "", "", 0, false
}

// emphasis matches **strong**, *em*, _em_, __strong__ and ~~strike~~ at
// s[i]. An underscore inside a word, as in snake_case, is literal.
func emphasis(s string, i int, style Style, url string) ([]Span, int, bool) {
	rest := s[i:]
	c := rest[0]
	n := len(rest) - len(strings.TrimLeft(rest, string(c)))
	switch {
	case c == '~' && n != 2:
		return nil, 0, false
	case n > 3:
		return []Span{{Text: rest[:n], Style: style, URL: url}}, n, true
	case n == 3:
		n = 2
	}
	delim := rest[:n]
	add := map[byte]Style{'*': Italic, '_': Italic, '~': Strike}[c]
	if n == 2 && c != '~' {
		add = Bold
	}
	opensWord := len(rest) > n && !unicode.IsSpace(rune(rest[n]))
	if !opensWord || (c == '_' && i > 0 && isWordByte(s[i-1])) {
		return []Span{{Text: delim, Style: style, URL: url}}, n, true
	}
	for j := n; j+n <= len(rest); j++ {
		if rest[j] == '\\' {
			j++
			continue
		}
		if !strings.HasPrefix(rest[j:], delim) || unicode.IsSpace(rune(rest[j-1])) {
			continue
		}
		if c == '_' && j+n < len(rest) && isWordByte(rest[j+n]) {
			continue
		}
		if n == 1 && j+1 < len(rest) && rest[j+1] == c {
			j++
			continue
		}
		return parseInline(rest[n:j], style|add, url), j + n, true
	}
	return []Span{{Text: delim, Style: style, URL: url}}, n, true
}

// trimURLTail drops the punctuation that ends a sentence rather than the URL,
// and a closing parenthesis the URL didn't open.
func trimURLTail(u string) string {
	for len(u) > 0 {
		last := u[len(u)-1]
		switch {
		case strings.IndexByte(".,:;!?'\"*_~", last) >= 0:
			u = u[:len(u)-1]
		case last == ')' && strings.Count(u, "(") < strings.Count(u, ")"):
			u = u[:len(u)-1]
		default:
			return u
		}
	}
	return u
}

func isASCIIPunct(b byte) bool {
	return b < utf8.RuneSelf && unicode.IsPunct(rune(b)) || strings.IndexByte("$+<=>^`|~", b) >= 0
}

func isWordByte(b byte) bool {
	return b >= utf8.RuneSelf || b == '_' || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b))
}

// appendSpan adds sp, merging it into the last span when they look alike.
func appendSpan(spans []Span, sp Span) []Span {
	if sp.Text == "" {
		return spans
	}
	if n := len(spans); n > 0 && spans[n-1].Style == sp.Style && spans[n-1].URL == sp.URL {
		merged := spans[n-1]
		merged.Text += sp.Text
		return append(spans[:n-1:n-1], merged)
	}
	return append(spans, sp)
}

// wrap breaks spans into lines at most width wide, at spaces where it can and
// inside a word only when the word is wider than a line. Runs of whitespace
// become one space, and a link broken across lines keeps its URL on every
// piece.
func wrap(spans []Span, width int) []Line {
	var lines []Line
	var line Line
	lineWidth := 0
	var space *Span
	for _, word := range words(spans) {
		if word.space != nil {
			space = word.space
			continue
		}
		w := lineWidth
		if w > 0 {
			w++
		}
		if w+word.width > width && lineWidth > 0 {
			lines = append(lines, line)
			line, lineWidth = nil, 0
		}
		if lineWidth > 0 && space != nil {
			line = appendLine(line, Span{Text: " ", Style: space.Style, URL: space.URL})
			lineWidth++
		}
		space = nil
		for _, sp := range word.spans {
			line = appendLine(line, sp)
		}
		lineWidth += word.width
		if lineWidth > width {
			pieces := hardWrap(line, width)
			lines = append(lines, pieces[:len(pieces)-1]...)
			line = pieces[len(pieces)-1]
			lineWidth = lineWidthOf(line)
		}
	}
	if len(line) > 0 {
		lines = append(lines, line)
	}
	return lines
}

func appendLine(l Line, sp Span) Line {
	return Line(appendSpan([]Span(l), sp))
}

type word struct {
	spans []Span
	width int
	// space is set, and spans empty, for the whitespace between words.
	space *Span
}

// words splits spans at whitespace, keeping each word's pieces in their own
// styles.
func words(spans []Span) []word {
	var out []word
	var cur word
	flush := func() {
		if len(cur.spans) > 0 {
			out = append(out, cur)
		}
		cur = word{}
	}
	for _, sp := range spans {
		start := 0
		for i, r := range sp.Text {
			if !unicode.IsSpace(r) {
				continue
			}
			if i > start {
				piece := Span{Text: sp.Text[start:i], Style: sp.Style, URL: sp.URL}
				cur.spans = append(cur.spans, piece)
				cur.width += lipgloss.Width(piece.Text)
			}
			flush()
			ws := sp
			out = append(out, word{space: &ws})
			start = i + utf8.RuneLen(r)
		}
		if start < len(sp.Text) {
			piece := Span{Text: sp.Text[start:], Style: sp.Style, URL: sp.URL}
			cur.spans = append(cur.spans, piece)
			cur.width += lipgloss.Width(piece.Text)
		}
	}
	flush()
	return out
}

// hardWrap cuts a line into pieces at most width wide, wherever the width
// runs out.
func hardWrap(l Line, width int) []Line {
	width = max(width, 1)
	var lines []Line
	var cur Line
	w := 0
	for _, sp := range l {
		var b strings.Builder
		for _, r := range sp.Text {
			rw := lipgloss.Width(string(r))
			if w+rw > width && w > 0 {
				cur = appendLine(cur, Span{Text: b.String(), Style: sp.Style, URL: sp.URL})
				lines = append(lines, cur)
				cur, w = nil, 0
				b.Reset()
			}
			b.WriteRune(r)
			w += rw
		}
		cur = appendLine(cur, Span{Text: b.String(), Style: sp.Style, URL: sp.URL})
	}
	return append(lines, cur)
}

func lineWidthOf(l Line) int {
	w := 0
	for _, sp := range l {
		w += lipgloss.Width(sp.Text)
	}
	return w
}
