package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	esc = 0x1b
	bel = 0x07
	del = 0x7f

	c1CSI = 0x9b
	c1OSC = 0x9d
	c1ST  = 0x9c
)

// sanitize strips terminal escape sequences and control characters from PR
// text, which is untrusted input. Newlines and tabs survive; everything else
// below 0x20, DEL and the C1 range is removed, along with the parameters of
// any sequence it introduces so no payload leaks through as visible text.
func sanitize(s string) string {
	runes := []rune(strings.ToValidUTF8(s, string(utf8.RuneError)))
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == esc:
			i = skipEscape(runes, i+1)
		case r == c1CSI:
			i = skipCSI(runes, i+1)
		case r == c1OSC || r == 0x90 || r == 0x98 || r == 0x9e || r == 0x9f:
			i = skipString(runes, i+1)
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == del || (r >= 0x80 && r <= 0x9f):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// skipEscape consumes what follows an ESC at runes[i] and returns the index of
// the sequence's last rune.
func skipEscape(runes []rune, i int) int {
	if i >= len(runes) {
		return i - 1
	}
	switch runes[i] {
	case '[':
		return skipCSI(runes, i+1)
	case ']', 'P', 'X', '^', '_':
		return skipString(runes, i+1)
	}
	for i < len(runes) && runes[i] >= 0x20 && runes[i] <= 0x2f {
		i++
	}
	if i >= len(runes) {
		return i - 1
	}
	return i
}

func skipCSI(runes []rune, i int) int {
	for i < len(runes) && runes[i] >= 0x20 && runes[i] <= 0x3f {
		i++
	}
	if i < len(runes) && runes[i] >= 0x40 && runes[i] <= 0x7e {
		return i
	}
	return i - 1
}

// skipString consumes an OSC, DCS, SOS, PM or APC body up to its terminator:
// BEL, ESC \ or the C1 string terminator.
func skipString(runes []rune, i int) int {
	for ; i < len(runes); i++ {
		switch {
		case runes[i] == bel || runes[i] == c1ST:
			return i
		case runes[i] == esc && i+1 < len(runes) && runes[i+1] == '\\':
			return i + 1
		}
	}
	return len(runes) - 1
}

// stripTransport sanitises every string in a JSON response before go-gh sees
// it. go-gh rewrites control characters as visible caret text (^[) above the
// transport, which neutralises them but leaves each sequence's payload on
// screen; stripping first means nothing of a sequence survives.
type stripTransport struct {
	next http.RoundTripper
}

func (s stripTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := s.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	body = sanitizeJSON(body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Del("Content-Length")
	return resp, nil
}

// sanitizeJSON returns body with every string value sanitised, or body as it
// was if it isn't JSON.
func sanitizeJSON(body []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return body
	}
	out, err := json.Marshal(sanitizeValue(v))
	if err != nil {
		return body
	}
	return out
}

func sanitizeValue(v any) any {
	switch v := v.(type) {
	case string:
		return sanitize(v)
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = sanitizeValue(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = sanitizeValue(e)
		}
		return out
	}
	return v
}
