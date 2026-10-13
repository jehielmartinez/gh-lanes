// Package clipboard hands text to the operating system's clipboard tool.
package clipboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ErrNoTool is returned when no clipboard tool is installed for this system.
var ErrNoTool = errors.New("no clipboard tool found")

// Copy puts text on the system clipboard. The text goes to the tool on stdin
// and never passes through a shell, since it may come from PR content.
func Copy(text string) error {
	name, args, ok := tool()
	if !ok {
		return ErrNoTool
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// tool is the first installed clipboard tool for this system and the
// arguments that make it read the clipboard's new content from stdin.
func tool() (string, []string, bool) {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			candidates = append(candidates, []string{"wl-copy"})
		}
		candidates = append(candidates,
			[]string{"xclip", "-selection", "clipboard"},
			[]string{"xsel", "--clipboard", "--input"},
		)
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err == nil {
			return c[0], c[1:], true
		}
	}
	return "", nil, false
}
