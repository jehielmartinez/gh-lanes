// Command lanes is a terminal kanban board for the open pull requests you
// authored. It runs as `gh lanes` or as a standalone `lanes` binary.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jehielmartinez/gh-lanes/internal/github"
	"github.com/jehielmartinez/gh-lanes/internal/store"
	"github.com/jehielmartinez/gh-lanes/internal/ui"
)

// version is stamped by the release build with -ldflags "-X main.version=...".
var version = ""

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lanes", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print the version and exit")
	configDir := flags.String("config", "", "config directory (default $XDG_CONFIG_HOME/lanes or ~/.config/lanes)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Fprintf(stdout, "lanes %s\n", resolveVersion())
		return 0
	}

	dir, err := store.Dir(*configDir)
	if err != nil {
		fmt.Fprintf(stderr, "lanes: %v\n", err)
		return 1
	}
	client, err := github.New(nil)
	if err != nil {
		fmt.Fprintf(stderr, "lanes: %v\n", err)
		return 1
	}

	app := ui.New(ui.Options{GitHub: client, ConfigDir: dir, Now: time.Now, After: time.After})
	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintf(stderr, "lanes: %v\n", err)
		return 1
	}
	return 0
}

// resolveVersion falls back to the module version Go stamps into the binary,
// so `go install` and the gh extension build report a version too.
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
