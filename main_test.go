package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const runMainEnv = "LANES_TEST_RUN_MAIN"

// TestMain lets tests run the real binary: the test executable re-runs itself
// with runMainEnv set and behaves as lanes.
func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func runLanes(t *testing.T, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), runMainEnv+"=1")
	cmd.Env = append(cmd.Env, env...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run lanes: %v", err)
	}
	return out.String(), errOut.String(), code
}

func TestVersionFlagPrintsVersion(t *testing.T) {
	stdout, _, code := runLanes(t, nil, "--version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout, "lanes ") || strings.TrimSpace(stdout) == "lanes" {
		t.Errorf("stdout = %q, want \"lanes <version>\"", stdout)
	}
}

func TestNotLoggedInTellsUserToRunGHAuthLogin(t *testing.T) {
	env := []string{
		"GH_CONFIG_DIR=" + t.TempDir(),
		"GH_PATH=" + filepath.Join(t.TempDir(), "no-gh"),
		"GH_TOKEN=", "GITHUB_TOKEN=", "GH_ENTERPRISE_TOKEN=", "GITHUB_ENTERPRISE_TOKEN=", "GH_HOST=",
		"XDG_CONFIG_HOME=" + t.TempDir(),
	}
	_, stderr, code := runLanes(t, env)
	if code == 0 {
		t.Errorf("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr, "gh auth login") {
		t.Errorf("stderr = %q, want it to tell the user to run gh auth login", stderr)
	}
}
