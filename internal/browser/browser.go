// Package browser hands URLs to the operating system's opener.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/jehielmartinez/gh-lanes/internal/links"
)

// Open opens an http or https URL in the default browser. The URL is the
// opener's only argument and never passes through a shell, since it comes
// from PR content.
func Open(url string) error {
	if !links.Openable(url) {
		return links.ErrNotOpenable
	}
	var opener string
	switch runtime.GOOS {
	case "darwin":
		opener = "open"
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		opener = "xdg-open"
	default:
		return fmt.Errorf("opening links isn't supported on %s", runtime.GOOS)
	}
	// With no stdin, stdout or stderr the opener reads nothing from the
	// terminal and draws nothing over the board.
	if err := exec.Command(opener, url).Run(); err != nil {
		return fmt.Errorf("%s: %w", opener, err)
	}
	return nil
}
